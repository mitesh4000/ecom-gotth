package upload_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitesh4000/pushti_sangar/server/internal/upload"
)

// Minimal valid file signatures. The upload package only sniffs headers, which
// is the security boundary being tested here.
var pngHeader = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
var jpegHeader = []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1}

func saveRequest(t *testing.T, field, filename string, data []byte) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	r := httptest.NewRequest("POST", "/upload", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return httptest.NewRecorder(), r
}

func TestSaveImageAcceptsPNGAndIgnoresClientName(t *testing.T) {
	dir := t.TempDir()
	_, r := saveRequest(t, "photo", "../../evil.png", append(pngHeader, bytes.Repeat([]byte("a"), 100)...))

	path, err := upload.SaveImage(r, "photo", dir, "test")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if strings.Contains(path, "..") {
		t.Fatalf("path must not traverse: %s", path)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("expected .png extension, got %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestSaveImageAcceptsJPEG(t *testing.T) {
	dir := t.TempDir()
	_, r := saveRequest(t, "photo", "photo.jpg", append(jpegHeader, bytes.Repeat([]byte("b"), 100)...))

	path, err := upload.SaveImage(r, "photo", dir, "test")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if filepath.Ext(path) != ".jpg" {
		t.Fatalf("expected .jpg, got %s", path)
	}
}

func TestSaveImageRejectsSVG(t *testing.T) {
	dir := t.TempDir()
	_, r := saveRequest(t, "photo", "evil.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))

	if _, err := upload.SaveImage(r, "photo", dir, "test"); err == nil {
		t.Fatal("expected SVG upload to be rejected")
	}
}

func TestSaveImageRejectsTextFileWithImageExtension(t *testing.T) {
	dir := t.TempDir()
	_, r := saveRequest(t, "photo", "not-really.png", []byte("this is plain text, not an image"))

	if _, err := upload.SaveImage(r, "photo", dir, "test"); err == nil {
		t.Fatal("expected non-image content to be rejected")
	}
}
