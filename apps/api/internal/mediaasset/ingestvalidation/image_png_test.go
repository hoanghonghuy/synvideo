package ingestvalidation

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type countingReaderAt struct {
	inner io.ReaderAt
	reads int
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	c.reads++
	return c.inner.ReadAt(p, off)
}

func appendPNGChunk(buf []byte, chunkType string, data []byte) []byte {
	chunk := make([]byte, 8+len(data)+4)
	binary.BigEndian.PutUint32(chunk[0:4], uint32(len(data)))
	chunk[4], chunk[5], chunk[6], chunk[7] = chunkType[0], chunkType[1], chunkType[2], chunkType[3]
	copy(chunk[8:], data)
	return append(buf, chunk...)
}

func appendPNGIEND(buf []byte) []byte {
	return appendPNGChunk(buf, "IEND", nil)
}

func pngWithManyPostPrefixChunks(postPrefixCount int) []byte {
	out := append([]byte(nil), adversarialPNGSignatureAndIHDR...)
	idat := bytes.Repeat([]byte{0x78, 0x9c, 0x63}, 4096)
	out = appendPNGChunk(out, "IDAT", idat)
	for range postPrefixCount {
		out = appendPNGChunk(out, "tEXt", nil)
	}
	return appendPNGIEND(out)
}

var adversarialPNGSignatureAndIHDR = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89,
}

func writePNGTestFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestValidatePNGUsesBoundedFileReads(t *testing.T) {
	data := pngWithManyPostPrefixChunks(100_000)
	if len(data) <= maxImageHeaderRead {
		t.Fatalf("fixture size %d must exceed header budget", len(data))
	}

	path := writePNGTestFile(t, data)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}

	prefix, err := readBoundedHeader(file)
	if err != nil {
		t.Fatalf("readBoundedHeader() error = %v", err)
	}

	counter := &countingReaderAt{inner: file}
	if err := validatePNG(imageProbe{file: counter, size: info.Size(), prefix: prefix}); err != nil {
		t.Fatalf("validatePNG() error = %v", err)
	}
	if counter.reads > 1 {
		t.Fatalf("ReadAt calls = %d, want <= 1", counter.reads)
	}
}

func TestValidatePNGRejectsExcessivePrefixChunkInspection(t *testing.T) {
	out := append([]byte(nil), adversarialPNGSignatureAndIHDR...)
	for range maxPNGChunkHeadersInspected + 1 {
		out = appendPNGChunk(out, "tEXt", nil)
	}
	out = appendPNGIEND(out)

	path := writePNGTestFile(t, out)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	prefix, err := readBoundedHeader(file)
	if err != nil {
		t.Fatalf("readBoundedHeader() error = %v", err)
	}

	err = validatePNG(imageProbe{file: file, size: info.Size(), prefix: prefix})
	if err != ErrMalformed {
		t.Fatalf("validatePNG() error = %v, want ErrMalformed", err)
	}
}
