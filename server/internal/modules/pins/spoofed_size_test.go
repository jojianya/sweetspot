package pins

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"
)

// TestProcessPhotosRejectsSpoofedSize proves the size gate is enforced on
// actual bytes, not the client-claimed FileHeader.Size. The header is mutated
// to a tiny value after parsing; the payload itself exceeds maxPhotoSize.
func TestProcessPhotosRejectsSpoofedSize(t *testing.T) {
	big := make([]byte, maxPhotoSize+1024)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("photos", "big.jpg")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := fw.Write(big); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	r := multipart.NewReader(&body, w.Boundary())
	form, err := r.ReadForm(64 << 20)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	defer form.RemoveAll()
	files := form.File["photos"]
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	// Spoof: claim a tiny size while the payload is oversize.
	files[0].Size = 1

	_, perr := processPhotos(files)
	if perr == nil {
		t.Fatalf("processPhotos accepted spoofed-size oversize payload, want 400")
	}
	if perr.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", perr.status)
	}
	if perr.msg != "one or more photos exceed 10MB" {
		t.Fatalf("msg = %q, want size error", perr.msg)
	}
}
