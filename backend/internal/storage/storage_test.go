package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/digithun/employee-portal/backend/internal/storage"
)

func pngBytes(size int) []byte {
	b := make([]byte, size)
	copy(b, []byte("\x89PNG\r\n\x1a\n"))
	return b
}

func jpegBytes(size int) []byte {
	b := make([]byte, size)
	copy(b, []byte("\xff\xd8\xff\xe0"))
	return b
}

func TestSaveAcceptsImages(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.NewLocal(dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}

	cases := map[string][]byte{
		".png": pngBytes(2048),
		".jpg": jpegBytes(2048),
	}
	for ext, content := range cases {
		url, err := s.Save(bytes.NewReader(content))
		if err != nil {
			t.Fatalf("Save(%s): %v", ext, err)
		}
		if !strings.HasPrefix(url, "/uploads/") || !strings.HasSuffix(url, ext) {
			t.Errorf("url = %q, want /uploads/<uuid>%s", url, ext)
		}
		// The whole body must reach disk, not just the sniffed header.
		written, err := os.ReadFile(filepath.Join(dir, filepath.Base(url)))
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if len(written) != len(content) {
			t.Errorf("wrote %d bytes, want %d", len(written), len(content))
		}
	}
}

func TestSaveGeneratesUniqueNames(t *testing.T) {
	s, _ := storage.NewLocal(t.TempDir())
	a, _ := s.Save(bytes.NewReader(pngBytes(64)))
	b, _ := s.Save(bytes.NewReader(pngBytes(64)))
	if a == b {
		t.Error("two uploads must not share a filename")
	}
}

func TestSaveRejectsNonImages(t *testing.T) {
	s, _ := storage.NewLocal(t.TempDir())

	// A script that merely claims to be a PNG through its filename gets nowhere: the
	// type is sniffed from the bytes.
	_, err := s.Save(strings.NewReader("#!/bin/sh\necho pwned\n"))
	if !errors.Is(err, storage.ErrUnsupportedType) {
		t.Errorf("expected ErrUnsupportedType, got %v", err)
	}

	if _, err := s.Save(bytes.NewReader([]byte("GIF89a"))); !errors.Is(err, storage.ErrUnsupportedType) {
		t.Errorf("GIF is outside the allowlist, got %v", err)
	}
	if _, err := s.Save(bytes.NewReader(nil)); !errors.Is(err, storage.ErrUnsupportedType) {
		t.Errorf("an empty upload should be rejected, got %v", err)
	}
}

func TestSaveLeavesNoFileWhenRejected(t *testing.T) {
	dir := t.TempDir()
	s, _ := storage.NewLocal(dir)
	s.Save(strings.NewReader("plain text"))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a rejected upload left %d files behind", len(entries))
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	s, _ := storage.NewLocal(dir)
	url, _ := s.Save(bytes.NewReader(pngBytes(64)))

	if err := s.Remove(url); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Base(url))); !os.IsNotExist(err) {
		t.Error("the file should be gone")
	}
	// Removing twice is not an error: the caller treats it as best effort.
	if err := s.Remove(url); err != nil {
		t.Errorf("removing a missing file should succeed, got %v", err)
	}
}

func TestRemoveIgnoresForeignAndTraversalURLs(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(outside, []byte("important"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sub := filepath.Join(dir, "uploads")
	s, _ := storage.NewLocal(sub)

	// Not one of ours: ignored.
	if err := s.Remove("https://cdn.example.com/a.png"); err != nil {
		t.Errorf("a foreign URL should be ignored, got %v", err)
	}
	// Traversal is stripped by filepath.Base before it reaches the filesystem.
	if err := s.Remove("/uploads/../keep.txt"); err != nil {
		t.Errorf("Remove: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("traversal deleted a file outside the upload dir: %v", err)
	}
}

func TestAllowlist(t *testing.T) {
	for _, ct := range []string{"image/jpeg", "image/png", "image/webp"} {
		if !storage.Allowed(ct) {
			t.Errorf("%s should be allowed", ct)
		}
	}
	for _, ct := range []string{"image/gif", "image/svg+xml", "text/html", "application/pdf", ""} {
		if storage.Allowed(ct) {
			t.Errorf("%s should not be allowed", ct)
		}
	}
}

func TestDetectContentTypeStripsParameters(t *testing.T) {
	if got := storage.DetectContentType([]byte("hello")); strings.Contains(got, ";") {
		t.Errorf("content type should have no parameters, got %q", got)
	}
}
