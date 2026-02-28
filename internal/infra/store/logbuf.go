package store

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"
)

// LogEntry represents a single log entry in the ring buffer.
type LogEntry struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Service string // "dhcp", "tftp", "http", "health", etc.
	MAC     string // optional, empty for non-client events
	Attrs   map[string]any
}

// LogBuffer is a thread-safe ring buffer for recent log entries.
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	size    int
	pos     int
	count   int
}

// NewLogBuffer creates a new ring buffer with the given capacity.
func NewLogBuffer(size int) *LogBuffer {
	if size < 1 {
		size = 1000
	}
	return &LogBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Add appends a log entry to the buffer, overwriting the oldest if full.
func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[b.pos] = entry
	b.pos = (b.pos + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

// Recent returns the most recent n log entries, newest first.
func (b *LogBuffer) Recent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n > b.count {
		n = b.count
	}
	if n == 0 {
		return nil
	}

	result := make([]LogEntry, n)
	for i := 0; i < n; i++ {
		idx := (b.pos - 1 - i + b.size) % b.size
		result[i] = b.entries[idx]
	}
	return result
}

// Filter returns entries matching the given criteria (newest first).
// Any filter field that is zero-valued is ignored.
func (b *LogBuffer) Filter(mac string, service string, level slog.Level, limit int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 {
		limit = b.count
	}

	var result []LogEntry
	for i := 0; i < b.count && len(result) < limit; i++ {
		idx := (b.pos - 1 - i + b.size) % b.size
		e := b.entries[idx]

		if mac != "" && e.MAC != mac {
			continue
		}
		if service != "" && e.Service != service {
			continue
		}
		if level > 0 && e.Level < level {
			continue
		}
		result = append(result, e)
	}
	return result
}

// SlogHandler returns an slog.Handler that writes to this log buffer.
func (b *LogBuffer) SlogHandler(service string) slog.Handler {
	return &logBufHandler{buf: b, service: service}
}

// logBufHandler implements slog.Handler and writes to a LogBuffer.
type logBufHandler struct {
	buf     *LogBuffer
	service string
	attrs   []slog.Attr
}

func (h *logBufHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *logBufHandler) Handle(_ context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
		Service: h.service,
		Attrs:   make(map[string]any),
	}

	// Copy pre-set attrs.
	for _, a := range h.attrs {
		entry.Attrs[a.Key] = a.Value.Any()
		if a.Key == "mac" {
			entry.MAC = formatMACAttr(a.Value)
		}
	}

	// Copy record attrs.
	r.Attrs(func(a slog.Attr) bool {
		entry.Attrs[a.Key] = a.Value.Any()
		if a.Key == "mac" {
			entry.MAC = formatMACAttr(a.Value)
		}
		return true
	})

	h.buf.Add(entry)
	return nil
}

func (h *logBufHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &logBufHandler{buf: h.buf, service: h.service, attrs: newAttrs}
}

func (h *logBufHandler) WithGroup(name string) slog.Handler {
	// Group support not needed for our use case.
	return h
}

func formatMACAttr(v slog.Value) string {
	if mac, ok := v.Any().(net.HardwareAddr); ok {
		return mac.String()
	}
	return v.String()
}
