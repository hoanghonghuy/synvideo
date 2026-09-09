package ingestvalidation_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/ingestvalidation"
)

const largeImagePadding = 70 * 1024

func largeJPEG(extra int) []byte {
	body := minimalJPEG[:len(minimalJPEG)-2]
	padding := bytes.Repeat([]byte{0x00}, extra)
	return append(append(body, padding...), 0xFF, 0xD9)
}

func pngWithoutIDAT() []byte {
	const idatOffset = 33
	iendIndex := len(minimalPNG) - 12
	out := make([]byte, 0, idatOffset+(len(minimalPNG)-iendIndex))
	out = append(out, minimalPNG[:idatOffset]...)
	return append(out, minimalPNG[iendIndex:]...)
}

func largePNGWithoutIDAT(extra int) []byte {
	textData := bytes.Repeat([]byte{0x41}, extra)
	textChunk := make([]byte, 8+len(textData)+4)
	binary.BigEndian.PutUint32(textChunk[0:4], uint32(len(textData)))
	textChunk[4], textChunk[5], textChunk[6], textChunk[7] = 't', 'E', 'X', 't'
	copy(textChunk[8:], textData)
	iendIndex := len(minimalPNG) - 12
	out := append([]byte(nil), minimalPNG[:33]...)
	out = append(out, textChunk...)
	return append(out, minimalPNG[iendIndex:]...)
}

func largePNG(extra int) []byte {
	iendIndex := len(minimalPNG) - 12
	idatData := bytes.Repeat([]byte{0x78, 0x9c, 0x63}, extra/3+1)[:extra]
	idatChunk := make([]byte, 8+len(idatData)+4)
	binary.BigEndian.PutUint32(idatChunk[0:4], uint32(len(idatData)))
	idatChunk[4], idatChunk[5], idatChunk[6], idatChunk[7] = 'I', 'D', 'A', 'T'
	copy(idatChunk[8:], idatData)
	return append(append(minimalPNG[:iendIndex], idatChunk...), minimalPNG[iendIndex:]...)
}

func largeWebP(extra int) []byte {
	payload := minimalWebP[20 : 20+binary.LittleEndian.Uint32(minimalWebP[16:20])]
	payload = append(payload, bytes.Repeat([]byte{0x00}, extra)...)
	chunkSize := uint32(len(payload))
	riffSize := uint32(4 + 8 + chunkSize)
	out := make([]byte, 8+riffSize)
	out[0], out[1], out[2], out[3] = 'R', 'I', 'F', 'F'
	binary.LittleEndian.PutUint32(out[4:8], riffSize)
	out[8], out[9], out[10], out[11] = 'W', 'E', 'B', 'P'
	out[12], out[13], out[14], out[15] = 'V', 'P', '8', ' '
	binary.LittleEndian.PutUint32(out[16:20], chunkSize)
	copy(out[20:], payload)
	return out
}

func largeGIF(extra int) []byte {
	body := minimalGIF[:len(minimalGIF)-1]
	return append(append(body, bytes.Repeat([]byte{0x00}, extra)...), 0x3B)
}

func largeAVIF(extra int) []byte {
	const mdatOffset = 32 + 249
	mdatSize := int(binary.BigEndian.Uint32(minimalAVIF[mdatOffset : mdatOffset+4]))
	payload := append(
		minimalAVIF[mdatOffset+8:mdatOffset+mdatSize],
		bytes.Repeat([]byte{0x00}, extra)...,
	)
	newMdatSize := 8 + len(payload)
	out := make([]byte, mdatOffset+newMdatSize)
	copy(out, minimalAVIF[:mdatOffset])
	binary.BigEndian.PutUint32(out[mdatOffset:mdatOffset+4], uint32(newMdatSize))
	copy(out[mdatOffset+8:], payload)
	return out
}

func TestValidatePNGRejectsWithoutIDAT(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	cases := map[string][]byte{
		"ihdr-iend-only":      pngWithoutIDAT(),
		"large-ihdr-tex-iend": largePNGWithoutIDAT(largeImagePadding),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTempFile(t, data)
			_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
				Kind: ingestvalidation.KindImage, MimeType: "image/png",
			})
			if !errors.Is(err, ingestvalidation.ErrMalformed) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateLargeImagesAboveHeaderBudgetRemainIngestible(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	cases := []struct {
		name string
		data []byte
		mime string
	}{
		{"png", largePNG(largeImagePadding), "image/png"},
		{"jpeg", largeJPEG(largeImagePadding), "image/jpeg"},
		{"webp", largeWebP(largeImagePadding), "image/webp"},
		{"gif", largeGIF(largeImagePadding), "image/gif"},
		{"avif", largeAVIF(largeImagePadding), "image/avif"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.data) <= 64*1024 {
				t.Fatalf("fixture size %d must exceed header budget", len(tc.data))
			}
			path := writeTempFile(t, tc.data)
			verified, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
				Kind: ingestvalidation.KindImage, MimeType: tc.mime,
			})
			if err != nil {
				t.Fatalf("ValidateFile() error = %v", err)
			}
			if verified.MimeType != tc.mime {
				t.Fatalf("verified mime = %q", verified.MimeType)
			}
		})
	}
}

func TestValidateLargeImagesRejectTruncatedOrIncompleteFiles(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	cases := map[string]struct {
		data []byte
		mime string
	}{
		"large-jpeg-missing-eoi":    {data: largeJPEG(largeImagePadding)[:len(largeJPEG(largeImagePadding))-1], mime: "image/jpeg"},
		"large-png-missing-iend":    {data: largePNG(largeImagePadding)[:len(largePNG(largeImagePadding))-8], mime: "image/png"},
		"large-webp-short-file":     {data: largeWebP(largeImagePadding)[:len(largeWebP(largeImagePadding))-100], mime: "image/webp"},
		"large-gif-missing-trailer": {data: largeGIF(largeImagePadding)[:len(largeGIF(largeImagePadding))-1], mime: "image/gif"},
		"large-avif-short-file":     {data: largeAVIF(largeImagePadding)[:len(largeAVIF(largeImagePadding))-100], mime: "image/avif"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if len(tc.data) <= 64*1024 {
				t.Fatalf("fixture size %d must exceed header budget", len(tc.data))
			}
			path := writeTempFile(t, tc.data)
			_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
				Kind: ingestvalidation.KindImage, MimeType: tc.mime,
			})
			if err == nil || errors.Is(err, ingestvalidation.ErrMismatch) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
