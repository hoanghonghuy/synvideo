package audio_test

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/audio"
)

func TestChunkTextExactInvariant(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
	}{
		{
			name:     "empty text",
			input:    "",
			maxRunes: 100,
		},
		{
			name:     "short text fits single chunk",
			input:    "Xin chào thế giới! Đây là một câu ngắn.",
			maxRunes: 100,
		},
		{
			name:     "multi-paragraph text splits at paragraphs",
			input:    "Đoạn văn thứ nhất có nhiều từ ngữ phong phú.\n\nĐoạn văn thứ hai tiếp tục mạch cảm xúc và chi tiết.\n\nĐoạn văn thứ ba kết thúc câu chuyện.",
			maxRunes: 60,
		},
		{
			name:     "sentence split with trailing spaces and punctuation",
			input:    "Câu đầu tiên rất rõ ràng. Câu thứ hai có thêm thông tin! Câu thứ ba có dấu chấm hỏi? Và câu cuối cùng kết thúc ở đây.",
			maxRunes: 35,
		},
		{
			name:     "words split on whitespace",
			input:    "Một chuỗi các từ không có dấu chấm phẩy nhưng có khoảng trắng đều đặn giữa các từ trong câu.",
			maxRunes: 25,
		},
		{
			name:     "long word exceeds limit forces rune safe split",
			input:    "SupercalifragilisticexpialidociousAndAnotherVeryLongWordThatCannotBeSplitNormally",
			maxRunes: 20,
		},
		{
			name:     "unicode characters and emojis preserved accurately",
			input:    "Chào mừng bạn đến với SynVideo 🎬🎥! Trải nghiệm video AI thông minh và đa ngôn ngữ: tiếng Việt, English, 日本語, 한국어, Español.",
			maxRunes: 40,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks := audio.ChunkText(tc.input, tc.maxRunes)
			if tc.input == "" {
				if len(chunks) != 0 {
					t.Fatalf("expected 0 chunks for empty input, got %d", len(chunks))
				}
				return
			}

			// Invariant 1: Concatenation of all chunks must equal original text exactly
			joined := strings.Join(chunks, "")
			if joined != tc.input {
				t.Fatalf("exact invariant broken:\nwant: %q\ngot:  %q", tc.input, joined)
			}

			// Invariant 2: No individual chunk exceeds maxRunes unless a single indivisible token exceeds it
			for i, chunk := range chunks {
				runeCount := 0
				for range chunk {
					runeCount++
				}
				if runeCount > tc.maxRunes {
					t.Fatalf("chunk %d exceeded maxRunes (%d > %d): %q", i, runeCount, tc.maxRunes, chunk)
				}
			}
		})
	}
}

func makeWAV(sampleRate uint32, numChannels uint16, pcmData []byte) []byte {
	buf := new(bytes.Buffer)
	bitsPerSample := uint16(16)
	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample/8)
	blockAlign := numChannels * (bitsPerSample / 8)
	dataLen := uint32(len(pcmData))
	riffLen := uint32(36 + dataLen)

	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, riffLen)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16)) // subchunk1 size
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))  // PCM format
	_ = binary.Write(buf, binary.LittleEndian, numChannels)
	_ = binary.Write(buf, binary.LittleEndian, sampleRate)
	_ = binary.Write(buf, binary.LittleEndian, byteRate)
	_ = binary.Write(buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(buf, binary.LittleEndian, bitsPerSample)
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataLen)
	buf.Write(pcmData)

	return buf.Bytes()
}

func TestWAVStitchAndDuration(t *testing.T) {
	sampleRate := uint32(16000)
	channels := uint16(1)
	// 1 second of 16-bit mono PCM = 32000 bytes
	pcmChunk1 := make([]byte, 32000)
	pcmChunk2 := make([]byte, 16000) // 0.5s

	wav1 := makeWAV(sampleRate, channels, pcmChunk1)
	wav2 := makeWAV(sampleRate, channels, pcmChunk2)

	dur1, err := audio.MeasureDuration("audio/wav", wav1)
	if err != nil {
		t.Fatalf("measure wav1 error: %v", err)
	}
	if dur1 < 0.99 || dur1 > 1.01 {
		t.Fatalf("expected wav1 duration ~1.0s, got %f", dur1)
	}

	stitched, durTotal, err := audio.StitchAudio("audio/wav", [][]byte{wav1, wav2})
	if err != nil {
		t.Fatalf("stitch wav error: %v", err)
	}
	if durTotal < 1.49 || durTotal > 1.51 {
		t.Fatalf("expected total duration ~1.5s, got %f", durTotal)
	}

	durStitched, err := audio.MeasureDuration("audio/wav", stitched)
	if err != nil {
		t.Fatalf("measure stitched wav error: %v", err)
	}
	if durStitched < 1.49 || durStitched > 1.51 {
		t.Fatalf("expected measured stitched duration ~1.5s, got %f", durStitched)
	}
}

func makeMP3Frame(bitrateKbps int, sampleRate int) []byte {
	var brIndex int
	switch bitrateKbps {
	case 128:
		brIndex = 9
	case 192:
		brIndex = 11
	default:
		brIndex = 9
	}

	var srIndex int
	switch sampleRate {
	case 44100:
		srIndex = 0
	case 48000:
		srIndex = 1
	case 32000:
		srIndex = 2
	default:
		srIndex = 0
	}

	header := []byte{0xFF, 0xFB, byte((brIndex << 4) | (srIndex << 2)), 0x00}
	frameLen := (144 * bitrateKbps * 1000) / sampleRate
	frame := make([]byte, frameLen)
	copy(frame, header)
	return frame
}

func TestMP3StitchAndDuration(t *testing.T) {
	frame := makeMP3Frame(128, 44100)
	var chunk1 []byte
	for i := 0; i < 40; i++ {
		chunk1 = append(chunk1, frame...)
	}
	var chunk2 []byte
	for i := 0; i < 20; i++ {
		chunk2 = append(chunk2, frame...)
	}

	dur1, err := audio.MeasureDuration("audio/mpeg", chunk1)
	if err != nil {
		t.Fatalf("measure mp3 chunk1 error: %v", err)
	}
	if dur1 < 1.0 || dur1 > 1.1 {
		t.Fatalf("expected mp3 chunk1 duration ~1.04s, got %f", dur1)
	}

	stitched, durTotal, err := audio.StitchAudio("audio/mpeg", [][]byte{chunk1, chunk2})
	if err != nil {
		t.Fatalf("stitch mp3 error: %v", err)
	}
	if durTotal < 1.5 || durTotal > 1.6 {
		t.Fatalf("expected stitched duration ~1.56s, got %f", durTotal)
	}

	durStitched, err := audio.MeasureDuration("audio/mpeg", stitched)
	if err != nil {
		t.Fatalf("measure stitched mp3 error: %v", err)
	}
	if durStitched < 1.5 || durStitched > 1.6 {
		t.Fatalf("expected measured stitched duration ~1.56s, got %f", durStitched)
	}
}
