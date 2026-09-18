package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"testing"
)

func newGzipWriter(buf *bytes.Buffer) *gzip.Writer { return gzip.NewWriter(buf) }

func encodeBase64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// decodeForTest reverses the gzip+base64 encoding used on the wire.
func decodeForTest(t *testing.T, encoded string) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("nao e base64: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("nao e gzip: %v", err)
	}
	defer zr.Close()

	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("falha ao descompactar: %v", err)
	}
	return string(data)
}
