package common

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
)

func TestGetAudioDuration_UnsupportedExt(t *testing.T) {
	_, err := GetAudioDuration(context.Background(), bytes.NewReader([]byte("x")), ".xyz")
	if err == nil {
		t.Error("unsupported extension should error")
	}
}

// TestGetAudioDuration_InvalidInputs drives every format dispatch branch with
// bytes that are NOT a valid stream, exercising the parser error paths (the
// bulk of each helper). The guarantee under test: a corrupt upload never
// panics and always surfaces an error (except MP3, whose decoder treats an
// empty stream as a zero-length track).
func TestGetAudioDuration_InvalidInputs(t *testing.T) {
	garbage := bytes.Repeat([]byte{0x00, 0x11, 0x22, 0x33}, 16) // 64 bytes
	// These decoders reliably reject a corrupt stream with an error (rather than
	// tolerating it or requiring a specific container the way the MP4/OGG libs
	// do), so we can assert the error contract directly.
	cases := []string{".wav", ".flac", ".aiff", ".aac"}
	for _, ext := range cases {
		if _, err := GetAudioDuration(context.Background(), bytes.NewReader(garbage), ext); err == nil {
			t.Errorf("%s with garbage input should error", ext)
		}
	}
	// MP3 decoder returns (0, nil) on a stream with no decodable frames.
	if d, err := GetAudioDuration(context.Background(), bytes.NewReader(garbage), ".mp3"); err != nil || d != 0 {
		t.Errorf("mp3 garbage: expected (0,nil), got (%v,%v)", d, err)
	}
}

func TestGetWebMDuration_EBMLHeader(t *testing.T) {
	// Valid EBML magic -> the simplified parser reports it needs a full parser.
	ebml := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, bytes.Repeat([]byte{0}, 32)...)
	if _, err := GetAudioDuration(context.Background(), bytes.NewReader(ebml), ".webm"); err == nil {
		t.Error("webm with EBML magic should error (no full parser)")
	}
	// Non-EBML bytes -> parse failure.
	if _, err := GetAudioDuration(context.Background(), bytes.NewReader([]byte("not-webm-data-here")), ".webm"); err == nil {
		t.Error("non-EBML webm should error")
	}
}

// TestGetWAVDuration_Valid feeds a hand-built 16-bit PCM mono WAV of exactly
// one second (8000 Hz) and asserts the computed duration.
func TestGetWAVDuration_Valid(t *testing.T) {
	const sampleRate = 8000
	const bitsPerSample = 16
	const numChannels = 1
	const seconds = 1
	dataBytes := sampleRate * (bitsPerSample / 8) * numChannels * seconds // 16000

	buf := &bytes.Buffer{}
	// RIFF header
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataBytes)) // chunk size
	buf.WriteString("WAVE")
	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))          // subchunk1 size
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))           // PCM
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels)) // channels
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))  // sample rate
	byteRate := sampleRate * numChannels * (bitsPerSample / 8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))                    // byte rate
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels*bitsPerSample/8)) // block align
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))               // bits per sample
	// data chunk
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataBytes))
	buf.Write(make([]byte, dataBytes))

	d, err := GetAudioDuration(context.Background(), bytes.NewReader(buf.Bytes()), ".wav")
	if err != nil {
		t.Fatalf("valid wav parse error: %v", err)
	}
	if d < 0.99 || d > 1.01 {
		t.Errorf("expected ~1.0s duration, got %f", d)
	}
}
