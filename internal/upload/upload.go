package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EnsureDirs(root string, subdirs ...string) error {
	for _, d := range subdirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// SaveImage saves the first file from form field and returns relative path used by API.
func SaveImage(r *http.Request, field, destDir, prefix string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", err // no file is OK for optional uploads; caller checks
	}
	defer file.Close()

	if !strings.HasPrefix(header.Header.Get("Content-Type"), "image/") &&
		!isImageExt(header.Filename) {
		return "", fmt.Errorf("only image files are allowed")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	name := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), prefix, ext)
	full := filepath.Join(destDir, name)

	dst, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return full, nil
}

func SaveImages(r *http.Request, field, destDir, prefix string, max int) ([]string, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
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
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(fh.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	name := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), prefix, ext)
	full := filepath.Join(destDir, name)
	dst, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return full, nil
}

func isImageExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return true
	default:
		return false
	}
}
