package ingestvalidation

import (
	"bytes"
	"encoding/binary"
	"io"
)

const maxImageHeaderRead = 64 * 1024

type imageFacts struct {
	mimeType string
}

func detectImage(data []byte) (imageFacts, error) {
	if len(data) < 12 {
		return imageFacts{}, ErrMalformed
	}
	switch {
	case isPNG(data):
		if err := validatePNGHeader(data); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/png"}, nil
	case isJPEG(data):
		if err := validateJPEGHeader(data); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/jpeg"}, nil
	case isGIF(data):
		if err := validateGIFHeader(data); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/gif"}, nil
	case isWebP(data):
		if err := validateWebPHeader(data); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/webp"}, nil
	case isAVIF(data):
		if err := validateAVIFHeader(data); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/avif"}, nil
	default:
		return imageFacts{}, ErrUnsupported
	}
}

func readBoundedHeader(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, maxImageHeaderRead)
	return io.ReadAll(limited)
}

func isPNG(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

func validatePNGHeader(data []byte) error {
	if len(data) < 33 {
		return ErrMalformed
	}
	if !bytes.Equal(data[12:16], []byte("IHDR")) {
		return ErrMalformed
	}
	chunkLen := binary.BigEndian.Uint32(data[8:12])
	if chunkLen != 13 {
		return ErrMalformed
	}
	width := binary.BigEndian.Uint32(data[16:20])
	height := binary.BigEndian.Uint32(data[20:24])
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	return nil
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
}

func validateJPEGHeader(data []byte) error {
	if len(data) < 4 {
		return ErrMalformed
	}
	marker := data[3]
	switch marker {
	case 0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF:
		if len(data) < 6 {
			return ErrMalformed
		}
		segmentLen := int(binary.BigEndian.Uint16(data[4:6]))
		if segmentLen < 2 || 4+segmentLen > len(data) {
			return ErrMalformed
		}
		return nil
	case 0xDB, 0xC0, 0xC1, 0xC2, 0xC4:
		return nil
	default:
		return ErrMalformed
	}
}

func isGIF(data []byte) bool {
	return len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a")))
}

func validateGIFHeader(data []byte) error {
	if len(data) < 13 {
		return ErrMalformed
	}
	width := binary.LittleEndian.Uint16(data[6:8])
	height := binary.LittleEndian.Uint16(data[8:10])
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	return nil
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

func validateWebPHeader(data []byte) error {
	if len(data) < 16 {
		return ErrMalformed
	}
	fourCC := data[12:16]
	switch {
	case bytes.Equal(fourCC, []byte("VP8 ")), bytes.Equal(fourCC, []byte("VP8L")), bytes.Equal(fourCC, []byte("VP8X")):
		return nil
	default:
		return ErrMalformed
	}
}

func isAVIF(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp"))
}

func validateAVIFHeader(data []byte) error {
	if len(data) < 16 {
		return ErrMalformed
	}
	boxSize := binary.BigEndian.Uint32(data[0:4])
	if boxSize < 16 || int(boxSize) > len(data) {
		return ErrMalformed
	}
	brands := string(data[8:boxSize])
	if !stringsContainsAVIFBrand(brands) {
		return ErrMalformed
	}
	return nil
}

func stringsContainsAVIFBrand(value string) bool {
	for _, brand := range []string{"avif", "avis", "mif1", "miaf"} {
		if len(value) >= len(brand) && containsBrand(value, brand) {
			return true
		}
	}
	return false
}

func containsBrand(value, brand string) bool {
	for i := 0; i+len(brand) <= len(value); i++ {
		if value[i:i+len(brand)] == brand {
			return true
		}
	}
	return false
}
