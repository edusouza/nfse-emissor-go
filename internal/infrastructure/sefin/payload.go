// Package sefin talks to the Sistema Nacional NFS-e web APIs.
package sefin

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
)

// maxDecompressedSize caps how much a response is allowed to expand to.
//
// The payloads are gzipped before base64 encoding, so a small response can
// decompress into an enormous one. The government is not an adversary, but a
// corrupted or misidentified payload should fail with a clear error rather than
// exhaust memory. A DPS or NFS-e document is a few tens of kilobytes; 32 MiB
// leaves several orders of magnitude of headroom.
const maxDecompressedSize = 32 << 20

// encodeGzipBase64 compresses data and encodes the result as standard base64,
// which is how XML documents travel to and from the national system.
func encodeGzipBase64(data []byte) (string, error) {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return "", fmt.Errorf("falha ao compactar o documento: %w", err)
	}
	// Close flushes the trailer; deferring it would encode a truncated stream.
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("falha ao finalizar a compactacao: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// decodeGzipBase64 reverses encodeGzipBase64.
func decodeGzipBase64(encoded string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("conteudo nao esta em base64 valido: %w", err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("conteudo nao esta compactado em gzip: %w", err)
	}
	defer zr.Close()

	// LimitReader caps the output; reading one extra byte tells us whether the
	// limit was reached rather than the stream simply ending there.
	data, err := io.ReadAll(io.LimitReader(zr, maxDecompressedSize+1))
	if err != nil {
		return nil, fmt.Errorf("falha ao descompactar o conteudo: %w", err)
	}
	if len(data) > maxDecompressedSize {
		return nil, fmt.Errorf("conteudo descompactado excede o limite de %d bytes", maxDecompressedSize)
	}

	return data, nil
}
