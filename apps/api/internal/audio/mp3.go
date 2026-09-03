package audio

import (
	"bytes"
	"errors"
	"fmt"
)

var (
	ErrInvalidMP3 = errors.New("invalid or empty MP3 stream")
)

var mp3BitratesV1L3 = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
var mp3BitratesV2L3 = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
var mp3SampleRatesV1 = []int{44100, 48000, 32000, 0}
var mp3SampleRatesV2 = []int{22050, 24000, 16000, 0}
var mp3SampleRatesV25 = []int{11025, 12000, 8000, 0}

type mp3FrameInfo struct {
	frameLength    int
	samplesInFrame int
	sampleRate     int
}

func parseMP3Frame(data []byte, offset int) (mp3FrameInfo, bool) {
	if offset+4 > len(data) {
		return mp3FrameInfo{}, false
	}
	b0 := data[offset]
	b1 := data[offset+1]
	b2 := data[offset+2]

	// Sync word: 11 bits set
	if b0 != 0xFF || (b1&0xE0) != 0xE0 {
		return mp3FrameInfo{}, false
	}

	versionBits := (b1 >> 3) & 0x03
	layerBits := (b1 >> 1) & 0x03

	if versionBits == 1 || layerBits == 0 { // reserved
		return mp3FrameInfo{}, false
	}

	isMPEG1 := versionBits == 3
	isMPEG2 := versionBits == 2
	isMPEG25 := versionBits == 0

	brIndex := int((b2 >> 4) & 0x0F)
	srIndex := int((b2 >> 2) & 0x03)
	padding := int((b2 >> 1) & 0x01)

	if brIndex == 0 || brIndex == 15 || srIndex == 3 {
		return mp3FrameInfo{}, false
	}

	var bitrate int
	var sampleRate int
	var samples int

	if isMPEG1 {
		bitrate = mp3BitratesV1L3[brIndex] * 1000
		sampleRate = mp3SampleRatesV1[srIndex]
		samples = 1152
	} else {
		bitrate = mp3BitratesV2L3[brIndex] * 1000
		if isMPEG2 {
			sampleRate = mp3SampleRatesV2[srIndex]
		} else if isMPEG25 {
			sampleRate = mp3SampleRatesV25[srIndex]
		}
		samples = 576
	}

	if sampleRate == 0 || bitrate == 0 {
		return mp3FrameInfo{}, false
	}

	var frameLen int
	if isMPEG1 {
		frameLen = (144 * bitrate / sampleRate) + padding
	} else {
		frameLen = (72 * bitrate / sampleRate) + padding
	}

	if frameLen <= 0 || offset+frameLen > len(data) {
		return mp3FrameInfo{}, false
	}

	return mp3FrameInfo{
		frameLength:    frameLen,
		samplesInFrame: samples,
		sampleRate:     sampleRate,
	}, true
}

func skipID3v2(data []byte) int {
	if len(data) >= 10 && string(data[0:3]) == "ID3" {
		size := (int(data[6]&0x7F) << 21) | (int(data[7]&0x7F) << 14) | (int(data[8]&0x7F) << 7) | int(data[9]&0x7F)
		totalTagLen := 10 + size
		if totalTagLen <= len(data) {
			return totalTagLen
		}
	}
	return 0
}

func measureMP3Duration(data []byte) (float64, error) {
	offset := skipID3v2(data)
	totalSamples := 0
	lastSampleRate := 44100
	frameCount := 0

	for offset < len(data) {
		info, ok := parseMP3Frame(data, offset)
		if !ok {
			// Try to find next sync byte
			offset++
			continue
		}
		frameCount++
		totalSamples += info.samplesInFrame
		lastSampleRate = info.sampleRate
		offset += info.frameLength
	}

	if frameCount == 0 || lastSampleRate == 0 {
		return 0, ErrInvalidMP3
	}

	return float64(totalSamples) / float64(lastSampleRate), nil
}

func stitchMP3(chunks [][]byte) ([]byte, float64, error) {
	if len(chunks) == 0 {
		return nil, 0, errors.New("no MP3 chunks to stitch")
	}
	if len(chunks) == 1 {
		dur, err := measureMP3Duration(chunks[0])
		return chunks[0], dur, err
	}

	var buf bytes.Buffer
	var totalDuration float64

	for i, chunk := range chunks {
		dur, err := measureMP3Duration(chunk)
		if err != nil {
			return nil, 0, fmt.Errorf("chunk %d: %w", i, err)
		}
		totalDuration += dur
		offset := skipID3v2(chunk)
		buf.Write(chunk[offset:])
	}

	return buf.Bytes(), totalDuration, nil
}
