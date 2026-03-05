// Package tftp implements the TFTP server that serves bootloader files.
package tftp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"bootforge/internal/domain"

	"github.com/pin/tftp/v3"
)

// AutoexecConfig holds the parameters needed to generate autoexec.ipxe dynamically.
// When set, TFTP requests for "autoexec.ipxe" return a generated script that chains
// to the HTTP boot menu using iPXE's built-in ${net0/mac} variable.
type AutoexecConfig struct {
	ServerIP string
	HTTPPort int
	ChainURL string // template with ${server_ip}, ${http_port}, ${mac} placeholders
}

// TFTPServer serves bootloader files via TFTP.
// Only files within the configured bootloader directory are served.
type TFTPServer struct {
	cfg       domain.TFTPConfig
	dataDir   string // absolute path to data directory
	blDir     string // bootloader subdirectory (relative to dataDir)
	autoexec  *AutoexecConfig
	logger    *slog.Logger
	server    *tftp.Server
	cancel    context.CancelFunc
}

// NewTFTPServer creates a new TFTP server.
func NewTFTPServer(cfg domain.TFTPConfig, dataDir string, blDir string, logger *slog.Logger) *TFTPServer {
	return &TFTPServer{
		cfg:     cfg,
		dataDir: dataDir,
		blDir:   blDir,
		logger:  logger,
	}
}

// SetAutoexec configures dynamic autoexec.ipxe generation.
func (s *TFTPServer) SetAutoexec(cfg AutoexecConfig) {
	s.autoexec = &cfg
}

// Name returns the service name.
func (s *TFTPServer) Name() string { return "tftp" }

// Start begins listening for TFTP requests.
func (s *TFTPServer) Start(ctx context.Context) error {
	_, s.cancel = context.WithCancel(ctx)

	s.server = tftp.NewServer(s.readHandler, nil)
	s.server.SetTimeout(s.cfg.Timeout)
	if s.cfg.BlockSize > 0 {
		s.server.SetBlockSize(s.cfg.BlockSize)
	}
	if s.cfg.Retries > 0 {
		s.server.SetRetries(s.cfg.Retries)
	}

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	conn, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	go func() {
		s.logger.Info("TFTP server started", "port", s.cfg.Port)
		if err := s.server.Serve(conn); err != nil {
			select {
			case <-ctx.Done():
				// Expected shutdown.
			default:
				s.logger.Error("TFTP server error", "error", err)
			}
		}
	}()

	return nil
}

// Stop shuts down the TFTP server.
func (s *TFTPServer) Stop(_ context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.server != nil {
		s.server.Shutdown()
	}
	return nil
}

// ReadHandler returns the TFTP read handler function for use in tests.
func (s *TFTPServer) ReadHandler() func(string, io.ReaderFrom) error {
	return s.readHandler
}

// readHandler serves TFTP read requests.
func (s *TFTPServer) readHandler(filename string, rf io.ReaderFrom) error {
	// Dynamic autoexec.ipxe generation.
	if s.autoexec != nil && filepath.Clean(filename) == "autoexec.ipxe" {
		return s.serveAutoexec(rf)
	}

	// Resolve the full path and ensure it's within the bootloader directory.
	fullPath, err := s.resolvePath(filename)
	if err != nil {
		s.logger.Warn("TFTP path rejected", "filename", filename, "error", err)
		return fmt.Errorf("access denied: %s", filename)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		s.logger.Debug("TFTP file not found", "filename", filename, "path", fullPath)
		return fmt.Errorf("file not found: %s", filename)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	// Set transfer size for the client.
	rf.(tftp.OutgoingTransfer).SetSize(stat.Size())

	s.logger.Info("TFTP transfer started",
		"filename", filename,
		"size", stat.Size(),
	)

	n, err := rf.ReadFrom(file)
	if err != nil {
		s.logger.Error("TFTP transfer failed", "filename", filename, "error", err)
		return err
	}

	s.logger.Info("TFTP transfer complete",
		"filename", filename,
		"bytes", n,
	)
	return nil
}

// serveAutoexec generates a dynamic autoexec.ipxe script that chains to the
// HTTP boot menu. Uses iPXE's built-in ${net0/mac} variable so the TFTP server
// doesn't need to know the client's MAC address.
func (s *TFTPServer) serveAutoexec(rf io.ReaderFrom) error {
	chainURL := s.autoexec.ChainURL
	chainURL = strings.ReplaceAll(chainURL, "${server_ip}", s.autoexec.ServerIP)
	chainURL = strings.ReplaceAll(chainURL, "${http_port}", fmt.Sprintf("%d", s.autoexec.HTTPPort))
	// Replace ${mac} with iPXE's runtime variable for the client's MAC address.
	chainURL = strings.ReplaceAll(chainURL, "${mac}", "${net0/mac}")

	script := "#!ipxe\nchain --autofree " + chainURL + "\n"

	reader := strings.NewReader(script)
	rf.(tftp.OutgoingTransfer).SetSize(int64(len(script)))

	s.logger.Info("TFTP serving dynamic autoexec.ipxe", "chain_url", chainURL)

	n, err := rf.ReadFrom(reader)
	if err != nil {
		s.logger.Error("TFTP autoexec transfer failed", "error", err)
		return err
	}

	s.logger.Debug("TFTP autoexec transfer complete", "bytes", n)
	return nil
}

// resolvePath validates that the requested file is within the bootloader directory.
// Returns the absolute path to the file or an error if the path escapes the directory.
func (s *TFTPServer) resolvePath(filename string) (string, error) {
	// Build the base bootloader directory path.
	baseDir := filepath.Join(s.dataDir, s.blDir)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base dir: %w", err)
	}

	// Clean the filename to prevent path traversal.
	cleaned := filepath.Clean(filename)
	fullPath := filepath.Join(absBase, cleaned)

	// Resolve to absolute and verify it's still within the base directory.
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("path traversal attempt: %s resolves to %s (outside %s)", filename, absPath, absBase)
	}

	return absPath, nil
}
