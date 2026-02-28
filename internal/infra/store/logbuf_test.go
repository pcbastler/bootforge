package store

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestLogBufferAddAndRecent(t *testing.T) {
	buf := NewLogBuffer(10)

	buf.Add(LogEntry{Message: "first", Time: time.Now()})
	buf.Add(LogEntry{Message: "second", Time: time.Now()})

	entries := buf.Recent(5)
	if len(entries) != 2 {
		t.Fatalf("Recent() count = %d, want 2", len(entries))
	}
	// Newest first.
	if entries[0].Message != "second" {
		t.Errorf("Recent()[0].Message = %q, want %q", entries[0].Message, "second")
	}
	if entries[1].Message != "first" {
		t.Errorf("Recent()[1].Message = %q, want %q", entries[1].Message, "first")
	}
}

func TestLogBufferOverflow(t *testing.T) {
	buf := NewLogBuffer(3) // Small buffer.

	for i := 0; i < 5; i++ {
		buf.Add(LogEntry{Message: string(rune('a' + i)), Time: time.Now()})
	}

	entries := buf.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("Recent() count = %d, want 3 (buffer size)", len(entries))
	}
	// Should have the 3 newest entries (c, d, e).
	if entries[0].Message != "e" {
		t.Errorf("Recent()[0].Message = %q, want %q", entries[0].Message, "e")
	}
	if entries[1].Message != "d" {
		t.Errorf("Recent()[1].Message = %q, want %q", entries[1].Message, "d")
	}
	if entries[2].Message != "c" {
		t.Errorf("Recent()[2].Message = %q, want %q", entries[2].Message, "c")
	}
}

func TestLogBufferRecentEmpty(t *testing.T) {
	buf := NewLogBuffer(10)

	entries := buf.Recent(5)
	if entries != nil {
		t.Errorf("Recent() should return nil for empty buffer, got %v", entries)
	}
}

func TestLogBufferRecentZero(t *testing.T) {
	buf := NewLogBuffer(10)
	buf.Add(LogEntry{Message: "test"})

	entries := buf.Recent(0)
	if entries != nil {
		t.Errorf("Recent(0) should return nil, got %v", entries)
	}
}

func TestLogBufferFilterByMAC(t *testing.T) {
	buf := NewLogBuffer(100)

	buf.Add(LogEntry{Message: "a", MAC: "aa:bb:cc:dd:ee:01", Service: "dhcp"})
	buf.Add(LogEntry{Message: "b", MAC: "aa:bb:cc:dd:ee:02", Service: "dhcp"})
	buf.Add(LogEntry{Message: "c", MAC: "aa:bb:cc:dd:ee:01", Service: "tftp"})

	entries := buf.Filter("aa:bb:cc:dd:ee:01", "", 0, 0)
	if len(entries) != 2 {
		t.Fatalf("Filter(mac) count = %d, want 2", len(entries))
	}
}

func TestLogBufferFilterByService(t *testing.T) {
	buf := NewLogBuffer(100)

	buf.Add(LogEntry{Message: "a", Service: "dhcp"})
	buf.Add(LogEntry{Message: "b", Service: "tftp"})
	buf.Add(LogEntry{Message: "c", Service: "dhcp"})

	entries := buf.Filter("", "dhcp", 0, 0)
	if len(entries) != 2 {
		t.Fatalf("Filter(service) count = %d, want 2", len(entries))
	}
}

func TestLogBufferFilterByLevel(t *testing.T) {
	buf := NewLogBuffer(100)

	buf.Add(LogEntry{Message: "debug", Level: slog.LevelDebug})
	buf.Add(LogEntry{Message: "info", Level: slog.LevelInfo})
	buf.Add(LogEntry{Message: "warn", Level: slog.LevelWarn})
	buf.Add(LogEntry{Message: "error", Level: slog.LevelError})

	entries := buf.Filter("", "", slog.LevelWarn, 0)
	if len(entries) != 2 {
		t.Fatalf("Filter(level>=warn) count = %d, want 2", len(entries))
	}
}

func TestLogBufferFilterWithLimit(t *testing.T) {
	buf := NewLogBuffer(100)

	for i := 0; i < 10; i++ {
		buf.Add(LogEntry{Message: "test", Service: "dhcp"})
	}

	entries := buf.Filter("", "", 0, 3)
	if len(entries) != 3 {
		t.Fatalf("Filter(limit=3) count = %d, want 3", len(entries))
	}
}

func TestLogBufferFilterCombined(t *testing.T) {
	buf := NewLogBuffer(100)

	buf.Add(LogEntry{Message: "match", MAC: "aa:bb:cc:dd:ee:01", Service: "dhcp", Level: slog.LevelWarn})
	buf.Add(LogEntry{Message: "wrong mac", MAC: "aa:bb:cc:dd:ee:02", Service: "dhcp", Level: slog.LevelWarn})
	buf.Add(LogEntry{Message: "wrong svc", MAC: "aa:bb:cc:dd:ee:01", Service: "tftp", Level: slog.LevelWarn})
	buf.Add(LogEntry{Message: "wrong lvl", MAC: "aa:bb:cc:dd:ee:01", Service: "dhcp", Level: slog.LevelDebug})

	entries := buf.Filter("aa:bb:cc:dd:ee:01", "dhcp", slog.LevelWarn, 0)
	if len(entries) != 1 {
		t.Fatalf("Filter(combined) count = %d, want 1", len(entries))
	}
	if entries[0].Message != "match" {
		t.Errorf("Filter(combined)[0].Message = %q, want %q", entries[0].Message, "match")
	}
}

func TestLogBufferSlogHandler(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := buf.SlogHandler("dhcp")
	logger := slog.New(handler)

	logger.Info("test message", "key", "value")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatalf("Recent() count = %d, want 1", len(entries))
	}
	if entries[0].Message != "test message" {
		t.Errorf("Message = %q, want %q", entries[0].Message, "test message")
	}
	if entries[0].Service != "dhcp" {
		t.Errorf("Service = %q, want %q", entries[0].Service, "dhcp")
	}
	if entries[0].Level != slog.LevelInfo {
		t.Errorf("Level = %v, want Info", entries[0].Level)
	}
	if entries[0].Attrs["key"] != "value" {
		t.Errorf("Attrs[key] = %v, want %q", entries[0].Attrs["key"], "value")
	}
}

func TestLogBufferSlogHandlerWithAttrs(t *testing.T) {
	buf := NewLogBuffer(100)
	handler := buf.SlogHandler("tftp")
	logger := slog.New(handler.WithAttrs([]slog.Attr{
		slog.String("mac", "aa:bb:cc:dd:ee:01"),
	}))

	logger.Info("transfer complete")

	entries := buf.Recent(1)
	if len(entries) != 1 {
		t.Fatal("no entries")
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:01" {
		t.Errorf("MAC = %q, want %q", entries[0].MAC, "aa:bb:cc:dd:ee:01")
	}
}

func TestLogBufferConcurrent(t *testing.T) {
	buf := NewLogBuffer(100)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			buf.Add(LogEntry{Message: "test", Time: time.Now()})
		}()
		go func() {
			defer wg.Done()
			buf.Recent(10)
		}()
	}
	wg.Wait()
}
