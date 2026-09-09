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

	offset := 8
	hasIDAT := false
	hasIEND := false
	for offset+8 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		chunkType := data[offset+4 : offset+8]
		chunkEnd := offset + 8 + chunkLen + 4
		if chunkLen < 0 || chunkEnd > len(data) {
			return ErrMalformed
		}
		switch {
		case bytes.Equal(chunkType, []byte("IDAT")):
			hasIDAT = true
		case bytes.Equal(chunkType, []byte("IEND")):
			hasIEND = true
		}
		if hasIEND {
			break
		}
		offset = chunkEnd
	}
	if !hasIDAT || !hasIEND {
		return ErrMalformed
	}
	return nil
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
}

func validateJPEGHeader(data []byte) error {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return ErrMalformed
	}

	offset := 2
	hasSOF := false

	for offset+1 < len(data) {
		if data[offset] != 0xFF {
			return ErrMalformed
		}
		marker := data[offset+1]
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

		if offset+2 > len(data) {
			return ErrMalformed
		}
		segmentLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if segmentLen < 2 {
			return ErrMalformed
		}
		segmentEnd := offset + segmentLen
		if segmentEnd > len(data) {
			return ErrMalformed
		}

		if marker == 0xDA {
			if !hasSOF {
				return ErrMalformed
			}
			if len(data) < 2 || data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
				return ErrMalformed
			}
			return nil
		}

		if isJPEGSOFMarker(marker) {
			if segmentLen < 8 {
				return ErrMalformed
			}
			height := binary.BigEndian.Uint16(data[offset+3 : offset+5])
			width := binary.BigEndian.Uint16(data[offset+5 : offset+7])
			if width == 0 || height == 0 {
				return ErrMalformed
			}
			hasSOF = true
		}

		offset = segmentEnd
	}

	if hasSOF && len(data) >= 2 && data[len(data)-2] == 0xFF && data[len(data)-1] == 0xD9 {
		return nil
	}
	return ErrMalformed
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

func validateGIFHeader(data []byte) error {
	if len(data) < 13 {
		return ErrMalformed
	}
	width := binary.LittleEndian.Uint16(data[6:8])
	height := binary.LittleEndian.Uint16(data[8:10])
	if width == 0 || height == 0 {
		return ErrMalformed
	}
	if data[len(data)-1] != 0x3B {
		return ErrMalformed
	}
	return nil
}

func isWebP(data []byte) bool {
	return len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

func validateWebPHeader(data []byte) error {
	if len(data) < 20 {
		return ErrMalformed
	}
	riffSize := binary.LittleEndian.Uint32(data[4:8])
	if int(riffSize)+8 > len(data) {
		return ErrMalformed
	}
	fourCC := data[12:16]
	chunkSize := binary.LittleEndian.Uint32(data[16:20])
	payloadStart := 20
	payloadEnd := payloadStart + int(chunkSize)
	if chunkSize == 0 || payloadEnd > len(data) || payloadEnd > int(riffSize)+8 {
		return ErrMalformed
	}
	payload := data[payloadStart:payloadEnd]
	switch {
	case bytes.Equal(fourCC, []byte("VP8 ")):
		return validateVP8Payload(payload)
	case bytes.Equal(fourCC, []byte("VP8L")):
		return validateVP8LPayload(payload)
	case bytes.Equal(fourCC, []byte("VP8X")):
		return validateVP8XPayload(payload)
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

func validateAVIFHeader(data []byte) error {
	offset := 0
	hasFTYP := false
	hasPayloadBox := false
	for offset+8 <= len(data) {
		boxSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if boxSize < 8 {
			return ErrMalformed
		}
		boxType := data[offset+4 : offset+8]
		boxEnd := offset + boxSize
		if boxEnd > len(data) {
			return ErrMalformed
		}
		switch {
		case bytes.Equal(boxType, []byte("ftyp")):
			if !stringsContainsAVIFBrand(string(data[offset+8 : boxEnd])) {
				return ErrMalformed
			}
			hasFTYP = true
		case bytes.Equal(boxType, []byte("meta")), bytes.Equal(boxType, []byte("mdat")), bytes.Equal(boxType, []byte("moov")):
			hasPayloadBox = true
		}
		offset = boxEnd
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
