package ingestvalidation

import (
	"bytes"
	"encoding/binary"
	"io"
)

const (
	maxImageHeaderRead          = 64 * 1024
	pngIENDTailSize             = 12
	maxPNGChunkHeadersInspected = 1024
)

type imageFacts struct {
	mimeType string
}

type imageProbe struct {
	file   io.ReaderAt
	size   int64
	prefix []byte
}

func detectImage(probe imageProbe) (imageFacts, error) {
	if probe.size < 1 || len(probe.prefix) < 12 {
		return imageFacts{}, ErrMalformed
	}
	switch {
	case isPNG(probe.prefix):
		if err := validatePNG(probe); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/png"}, nil
	case isJPEG(probe.prefix):
		if err := validateJPEG(probe); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/jpeg"}, nil
	case isGIF(probe.prefix):
		if err := validateGIF(probe); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/gif"}, nil
	case isWebP(probe.prefix):
		if err := validateWebP(probe); err != nil {
			return imageFacts{}, err
		}
		return imageFacts{mimeType: "image/webp"}, nil
	case isAVIF(probe.prefix):
		if err := validateAVIF(probe); err != nil {
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

func readFileRange(file io.ReaderAt, size, offset, length int64) ([]byte, error) {
	if offset < 0 || length < 0 || offset+length > size {
		return nil, ErrMalformed
	}
	buf := make([]byte, length)
	n, err := file.ReadAt(buf, offset)
	if err != nil {
		return nil, ErrMalformed
	}
	if int64(n) != length {
		return nil, ErrMalformed
	}
	return buf, nil
}

func isPNG(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

func validatePNG(probe imageProbe) error {
	prefix := probe.prefix
	if len(prefix) < 33 || !bytes.Equal(prefix[12:16], []byte("IHDR")) {
		return ErrMalformed
	}
	ihdrLen := binary.BigEndian.Uint32(prefix[8:12])
	if ihdrLen != 13 {
		return ErrMalformed
	}
	width := binary.BigEndian.Uint32(prefix[16:20])
	height := binary.BigEndian.Uint32(prefix[20:24])
	if width == 0 || height == 0 {
		return ErrMalformed
	}

	hasIDAT := false
	inspected := 0
	prefixLimit := len(prefix)
	offset := 8
	for offset+8 <= prefixLimit {
		if inspected >= maxPNGChunkHeadersInspected {
			return ErrMalformed
		}
		inspected++

		chunkLen := int64(binary.BigEndian.Uint32(prefix[offset : offset+4]))
		chunkType := prefix[offset+4 : offset+8]
		chunkEnd := int64(offset) + 8 + chunkLen + 4
		if chunkEnd > probe.size {
			return ErrMalformed
		}

		switch {
		case bytes.Equal(chunkType, []byte("IDAT")):
			hasIDAT = true
		case bytes.Equal(chunkType, []byte("IEND")):
			if !hasIDAT || chunkEnd != probe.size {
				return ErrMalformed
			}
			return nil
		}
		if hasIDAT {
			break
		}

		if chunkEnd > int64(prefixLimit) {
			break
		}
		if chunkEnd <= int64(offset) {
			return ErrMalformed
		}
		offset = int(chunkEnd)
	}

	if !hasIDAT {
		return ErrMalformed
	}
	if probe.size < pngIENDTailSize {
		return ErrMalformed
	}
	tail, err := readFileRange(probe.file, probe.size, probe.size-pngIENDTailSize, pngIENDTailSize)
	if err != nil {
		return err
	}
	if binary.BigEndian.Uint32(tail[0:4]) != 0 || !bytes.Equal(tail[4:8], []byte("IEND")) {
		return ErrMalformed
	}
	return nil
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
}

func validateJPEG(probe imageProbe) error {
	prefix := probe.prefix
	if len(prefix) < 4 || prefix[0] != 0xFF || prefix[1] != 0xD8 {
		return ErrMalformed
	}

	offset := 2
	hasSOF := false
	for offset+1 < len(prefix) {
		if prefix[offset] != 0xFF {
			return ErrMalformed
		}
		marker := prefix[offset+1]
		offset += 2

		switch marker {
		case 0xD8:
			continue
		case 0xD9:
			if !hasSOF {
				return ErrMalformed
			}
			return nil
		case 0x01, 0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7:
			continue
		}
		if marker == 0x00 {
			return ErrMalformed
		}
		if offset+2 > len(prefix) {
			return ErrMalformed
		}
		segmentLen := int(binary.BigEndian.Uint16(prefix[offset : offset+2]))
		if segmentLen < 2 {
			return ErrMalformed
		}
		segmentEnd := int64(offset + segmentLen)
		if segmentEnd > probe.size {
			return ErrMalformed
		}
		if segmentEnd > int64(len(prefix)) {
			return ErrMalformed
		}

		if marker == 0xDA {
			if !hasSOF {
				return ErrMalformed
			}
			return validateJPEGTrailer(probe)
		}

		if isJPEGSOFMarker(marker) {
			if segmentLen < 8 {
				return ErrMalformed
			}
			height := binary.BigEndian.Uint16(prefix[offset+3 : offset+5])
			width := binary.BigEndian.Uint16(prefix[offset+5 : offset+7])
			if width == 0 || height == 0 {
				return ErrMalformed
			}
			hasSOF = true
		}

		offset = int(segmentEnd)
	}

	if !hasSOF {
		return ErrMalformed
	}
	return validateJPEGTrailer(probe)
}

func validateJPEGTrailer(probe imageProbe) error {
	if probe.size < 2 {
		return ErrMalformed
	}
	tail, err := readFileRange(probe.file, probe.size, probe.size-2, 2)
	if err != nil {
		return err
	}
	if tail[0] != 0xFF || tail[1] != 0xD9 {
		return ErrMalformed
	}
	return nil
}

func isJPEGSOFMarker(marker byte) bool {
	switch marker {
	case 0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF:
		return true
	default:
		return false
	}
}

func isGIF(data []byte) bool {
	return len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a")))
}

func validateGIF(probe imageProbe) error {
	prefix := probe.prefix
	if len(prefix) < 13 {
		return ErrMalformed
	}
	width := binary.LittleEndian.Uint16(prefix[6:8])
	height := binary.LittleEndian.Uint16(prefix[8:10])
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	if probe.size < 1 {
		return ErrMalformed
	}
	tail, err := readFileRange(probe.file, probe.size, probe.size-1, 1)
	if err != nil {
		return err
	}
	if tail[0] != 0x3B {
		return ErrMalformed
	}
	return nil
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

func validateWebP(probe imageProbe) error {
	prefix := probe.prefix
	if len(prefix) < 20 {
		return ErrMalformed
	}
	riffSize := int64(binary.LittleEndian.Uint32(prefix[4:8]))
	declaredFileSize := riffSize + 8
	if declaredFileSize != probe.size {
		return ErrMalformed
	}
	fourCC := prefix[12:16]
	chunkSize := binary.LittleEndian.Uint32(prefix[16:20])
	if chunkSize == 0 {
		return ErrMalformed
	}
	payloadStart := 20
	payloadEnd := int64(payloadStart) + int64(chunkSize)
	if payloadEnd > probe.size {
		return ErrMalformed
	}
	available := prefix[payloadStart:]
	if int64(len(available)) > int64(chunkSize) {
		available = available[:chunkSize]
	}
	switch {
	case bytes.Equal(fourCC, []byte("VP8 ")):
		return validateVP8Payload(available)
	case bytes.Equal(fourCC, []byte("VP8L")):
		return validateVP8LPayload(available)
	case bytes.Equal(fourCC, []byte("VP8X")):
		return validateVP8XPayload(available)
	default:
		return ErrMalformed
	}
}

func validateVP8Payload(payload []byte) error {
	if len(payload) < 10 {
		return ErrMalformed
	}
	if payload[0]&0x01 != 0 {
		return ErrMalformed
	}
	if payload[3] != 0x9D || payload[4] != 0x01 || payload[5] != 0x2A {
		return ErrMalformed
	}
	width := int(payload[6]) | int(payload[7]&0x3F)<<8
	height := int(payload[8]) | int(payload[9]&0x3F)<<8
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	return nil
}

func validateVP8LPayload(payload []byte) error {
	if len(payload) < 5 || payload[0] != 0x2F {
		return ErrMalformed
	}
	width := 1 + int(payload[1]) + int(payload[2]&0x3F)<<8
	height := 1 + int(payload[3]) + int(payload[4]&0x3F)<<8
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	return nil
}

func validateVP8XPayload(payload []byte) error {
	if len(payload) < 10 {
		return ErrMalformed
	}
	width := 1 + int(payload[4]) + int(payload[5])<<8 + int(payload[6]&0x0F)<<16
	height := 1 + int(payload[7]) + int(payload[8])<<8 + int(payload[9]&0x0F)<<16
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	return nil
}

func isAVIF(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp"))
}

func validateAVIF(probe imageProbe) error {
	prefix := probe.prefix
	offset := 0
	hasFTYP := false
	hasPayloadBox := false
	for offset+8 <= len(prefix) {
		boxSize := int(binary.BigEndian.Uint32(prefix[offset : offset+4]))
		if boxSize < 8 {
			return ErrMalformed
		}
		boxType := prefix[offset+4 : offset+8]
		boxEnd := int64(offset + boxSize)
		if boxEnd > probe.size {
			return ErrMalformed
		}
		switch {
		case bytes.Equal(boxType, []byte("ftyp")):
			if boxEnd > int64(len(prefix)) {
				return ErrMalformed
			}
			if !stringsContainsAVIFBrand(string(prefix[offset+8 : boxEnd])) {
				return ErrMalformed
			}
			hasFTYP = true
		case bytes.Equal(boxType, []byte("meta")), bytes.Equal(boxType, []byte("mdat")), bytes.Equal(boxType, []byte("moov")):
			hasPayloadBox = true
		}
		if boxEnd > int64(len(prefix)) {
			break
		}
		offset += boxSize
	}
	if !hasFTYP || !hasPayloadBox {
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
