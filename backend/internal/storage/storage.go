// Package storage persists uploaded avatar files. The interface exists so the service
// layer does not care whether bytes land on a volume or in object storage (D5).
package storage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var ErrUnsupportedType = errors.New("unsupported image type: use JPEG, PNG or WebP")

// allowedTypes maps a sniffed content type to the extension we store it under.
var allowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type Storage interface {
	// Save writes r under a generated name and returns the public URL.
	Save(r io.Reader) (string, error)
	// Remove deletes a previously saved URL. Missing files are not an error.
	Remove(url string) error
}

// Local writes files into a directory served at URLPrefix.
type Local struct {
	Dir       string
	URLPrefix string
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &Local{Dir: dir, URLPrefix: "/uploads/"}, nil
}

func (l *Local) Save(r io.Reader) (string, error) {
	// Sniff the real type from the bytes rather than trusting the client header (FR-3.2).
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	head = head[:n]

	ext, ok := allowedTypes[contentType(head)]
	if !ok {
		return "", ErrUnsupportedType
	}

	name := uuid.NewString() + ext
	path := filepath.Join(l.Dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(head); err != nil {
		os.Remove(path)
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		os.Remove(path)
		return "", err
	}
	return l.URLPrefix + name, nil
}

func (l *Local) Remove(url string) error {
	if !strings.HasPrefix(url, l.URLPrefix) {
		return nil
	}
	// Base() strips any traversal attempt before the name reaches the filesystem.
	name := filepath.Base(strings.TrimPrefix(url, l.URLPrefix))
	if name == "." || name == string(filepath.Separator) {
		return nil
	}
	err := os.Remove(filepath.Join(l.Dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func contentType(head []byte) string {
	ct := http.DetectContentType(head)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}

// DetectContentType is exported for tests that check the allowlist directly.
func DetectContentType(head []byte) string { return contentType(head) }

// Allowed reports whether a sniffed type is accepted.
func Allowed(ct string) bool { _, ok := allowedTypes[ct]; return ok }
