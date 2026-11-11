package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxImageBytes is the maximum size of a single uploaded image.
	MaxImageBytes = 8 << 20
	// MaxFormMemory is the in-memory part of a multipart request. Larger parts
	// are spooled to temporary files by net/http.
	MaxFormMemory = 8 << 20
)

// allowedImageTypes maps sniffed MIME types to a safe file extension. SVG and
// BMP are intentionally excluded: SVG can contain active content and neither
// format is needed by the existing UI.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func EnsureDirs(root string, subdirs ...string) error {
	for _, d := range subdirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// SaveImage saves the first file from form field and returns the stored path
// used by the API. Missing files are reported as an error; callers treat
// uploads as optional and should ignore fs.ErrNotExist-style failures.
func SaveImage(r *http.Request, field, destDir, prefix string) (string, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return saveReader(file, destDir, prefix)
}

func SaveImages(r *http.Request, field, destDir, prefix string, max int) ([]string, error) {
	if err := r.ParseMultipartForm(MaxFormMemory); err != nil {
		return nil, err
	}
	files := r.MultipartForm.File[field]
	if len(files) == 0 {
		return nil, nil
	}
	if max > 0 && len(files) > max {
		files = files[:max]
	}
	var paths []string
	for _, fh := range files {
		p, err := saveFileHeader(fh, destDir, prefix)
		if err != nil {
			return paths, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func saveFileHeader(fh *multipart.FileHeader, destDir, prefix string) (string, error) {
	if fh.Size > MaxImageBytes {
		return "", fmt.Errorf("image exceeds %d bytes", MaxImageBytes)
	}
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	return saveReader(src, destDir, prefix)
}

// saveReader validates the actual file signature, ignores the client-supplied
// filename and writes the file with a random name.
func saveReader(src io.Reader, destDir, prefix string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	limited := io.LimitReader(src, MaxImageBytes+1)
	head := make([]byte, 512)
	n, err := io.ReadFull(limited, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	head = head[:n]
	if len(head) == 0 {
		return "", fmt.Errorf("empty upload")
	}
	mimeType := http.DetectContentType(head)
	ext, ok := allowedImageTypes[mimeType]
	if !ok {
		return "", fmt.Errorf("unsupported image type %q", mimeType)
	}

	name, err := randomName(prefix, ext)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(destDir, ".upload-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(head); err != nil {
		cleanup()
		return "", err
	}
	written, err := io.Copy(tmp, limited)
	if err != nil {
		cleanup()
		return "", err
	}
	if int64(len(head))+written > MaxImageBytes {
		cleanup()
		return "", fmt.Errorf("image exceeds %d bytes", MaxImageBytes)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}

	full := filepath.Join(destDir, name)
	if err := os.Rename(tmpName, full); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	return full, nil
}

func randomName(prefix, ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	safePrefix := sanitize(prefix)
	if safePrefix == "" {
		safePrefix = "image"
	}
	return safePrefix + "_" + hex.EncodeToString(b) + ext, nil
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() > 40 {
		return b.String()[:40]
	}
	return b.String()
}
