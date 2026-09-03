package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrInvalidWAVHeader  = errors.New("invalid WAV header")
	ErrUnsupportedWAV    = errors.New("unsupported WAV format")
	ErrWAVFormatMismatch = errors.New("WAV chunk formats do not match")
)

type wavHeaderInfo struct {
	numChannels   uint16
	sampleRate    uint32
	byteRate      uint32
	blockAlign    uint16
	bitsPerSample uint16
	dataOffset    int
	dataLength    int
}

func parseWAV(data []byte) (wavHeaderInfo, error) {
	if len(data) < 44 {
		return wavHeaderInfo{}, ErrInvalidWAVHeader
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return wavHeaderInfo{}, ErrInvalidWAVHeader
	}

	info := wavHeaderInfo{}
	offset := 12
	hasFmt := false
	hasData := false

	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8

		if chunkID == "fmt " {
			if int(chunkSize) < 16 || offset+int(chunkSize) > len(data) {
				return wavHeaderInfo{}, ErrInvalidWAVHeader
			}
			audioFormat := binary.LittleEndian.Uint16(data[offset : offset+2])
			if audioFormat != 1 && audioFormat != 3 { // 1 = PCM, 3 = IEEE float
				return wavHeaderInfo{}, ErrUnsupportedWAV
			}
			info.numChannels = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
			info.sampleRate = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
			info.byteRate = binary.LittleEndian.Uint32(data[offset+8 : offset+12])
			info.blockAlign = binary.LittleEndian.Uint16(data[offset+12 : offset+14])
			info.bitsPerSample = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
			hasFmt = true
		} else if chunkID == "data" {
			info.dataOffset = offset
			dataLen := int(chunkSize)
			if offset+dataLen > len(data) {
				dataLen = len(data) - offset
			}
			info.dataLength = dataLen
			hasData = true
			break
		}

		offset += int(chunkSize)
	}

	if !hasFmt || !hasData || info.byteRate == 0 {
		return wavHeaderInfo{}, ErrInvalidWAVHeader
	}

	return info, nil
}

func measureWAVDuration(data []byte) (float64, error) {
	info, err := parseWAV(data)
	if err != nil {
		return 0, err
	}
	duration := float64(info.dataLength) / float64(info.byteRate)
	return duration, nil
}

func stitchWAV(chunks [][]byte) ([]byte, float64, error) {
	if len(chunks) == 0 {
		return nil, 0, errors.New("no WAV chunks to stitch")
	}
	if len(chunks) == 1 {
		dur, err := measureWAVDuration(chunks[0])
		return chunks[0], dur, err
	}

	firstInfo, err := parseWAV(chunks[0])
	if err != nil {
		return nil, 0, fmt.Errorf("chunk 0: %w", err)
	}

	var totalDataLen int
	var rawDataBuffers [][]byte

	for i, chunk := range chunks {
		info, err := parseWAV(chunk)
		if err != nil {
			return nil, 0, fmt.Errorf("chunk %d: %w", i, err)
		}
		if info.sampleRate != firstInfo.sampleRate || info.numChannels != firstInfo.numChannels || info.bitsPerSample != firstInfo.bitsPerSample {
			return nil, 0, fmt.Errorf("chunk %d format mismatch: %w", i, ErrWAVFormatMismatch)
		}
		rawData := chunk[info.dataOffset : info.dataOffset+info.dataLength]
		rawDataBuffers = append(rawDataBuffers, rawData)
		totalDataLen += len(rawData)
	}

	buf := new(bytes.Buffer)
	riffLen := uint32(36 + totalDataLen)

	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, riffLen)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	_ = binary.Write(buf, binary.LittleEndian, firstInfo.numChannels)
	_ = binary.Write(buf, binary.LittleEndian, firstInfo.sampleRate)
	_ = binary.Write(buf, binary.LittleEndian, firstInfo.byteRate)
	_ = binary.Write(buf, binary.LittleEndian, firstInfo.blockAlign)
	_ = binary.Write(buf, binary.LittleEndian, firstInfo.bitsPerSample)
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(totalDataLen))

	for _, raw := range rawDataBuffers {
		buf.Write(raw)
	}

	duration := float64(totalDataLen) / float64(firstInfo.byteRate)
	return buf.Bytes(), duration, nil
}
