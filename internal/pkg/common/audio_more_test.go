package common

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
)

// buildFLAC hand-assembles a minimal, valid FLAC stream: the "fLaC" signature
// followed by a single (last) STREAMINFO metadata block. That block alone
// carries the sample rate and total-sample count getFLACDuration divides to get
// the duration, so no audio frames are needed.
func buildFLAC(sampleRate uint32, channels uint8, bitsPerSample uint8, totalSamples uint64) []byte {
	out := &bytes.Buffer{}
	out.WriteString("fLaC")

	// Metadata block header: last-block flag (bit7=1) | block type 0 (STREAMINFO),
	// then a 24-bit big-endian body length of 34.
	out.WriteByte(0x80) // last block, type STREAMINFO
	out.WriteByte(0x00)
	out.WriteByte(0x00)
	out.WriteByte(34)

	// STREAMINFO body (34 bytes).
	_ = binary.Write(out, binary.BigEndian, uint16(4096)) // min block size (>=16)
	_ = binary.Write(out, binary.BigEndian, uint16(4096)) // max block size (>=16)
	out.Write([]byte{0x00, 0x00, 0x00})                   // min frame size (24 bits, 0=unknown)
	out.Write([]byte{0x00, 0x00, 0x00})                   // max frame size (24 bits, 0=unknown)

	// 64-bit packed field: sampleRate(20) | channels-1(3) | bps-1(5) | totalSamples(36).
	var packed uint64
	packed |= uint64(sampleRate&0xFFFFF) << 44
	packed |= uint64((channels-1)&0x7) << 41
	packed |= uint64((bitsPerSample-1)&0x1F) << 36
	packed |= totalSamples & 0xFFFFFFFFF
	_ = binary.Write(out, binary.BigEndian, packed)

	out.Write(make([]byte, 16)) // MD5 checksum (unchecked)
	return out.Bytes()
}

// TestGetFLACDuration_Valid feeds a hand-built STREAMINFO-only FLAC of exactly
// one second (8000 Hz, 8000 samples) and asserts getFLACDuration's
// NSamples/SampleRate math. This exercises the parse-success branch that garbage
// input never reaches.
func TestGetFLACDuration_Valid(t *testing.T) {
	flacBytes := buildFLAC(8000, 1, 16, 8000)
	d, err := GetAudioDuration(context.Background(), bytes.NewReader(flacBytes), ".flac")
	if err != nil {
		t.Fatalf("valid flac parse error: %v", err)
	}
	if d < 0.99 || d > 1.01 {
		t.Errorf("expected ~1.0s duration, got %f", d)
	}

	// A 2-second 44.1 kHz stereo clip proves the math tracks sample rate/count,
	// not a hard-coded 1.0.
	flac2 := buildFLAC(44100, 2, 16, 88200)
	d2, err := GetAudioDuration(context.Background(), bytes.NewReader(flac2), ".flac")
	if err != nil {
		t.Fatalf("valid flac(2s) parse error: %v", err)
	}
	if d2 < 1.99 || d2 > 2.01 {
		t.Errorf("expected ~2.0s duration, got %f", d2)
	}
}

// buildMP4 hand-assembles a minimal MP4 whose moov/mvhd box carries a timescale
// and duration. getM4ADuration divides Duration/Timescale, so a full track table
// is unnecessary for the duration probe.
func buildMP4(timescale, duration uint32) []byte {
	mvhdBody := &bytes.Buffer{}
	mvhdBody.Write([]byte{0x00, 0x00, 0x00, 0x00})          // version 0 + flags
	_ = binary.Write(mvhdBody, binary.BigEndian, uint32(0)) // creation time
	_ = binary.Write(mvhdBody, binary.BigEndian, uint32(0)) // modification time
	_ = binary.Write(mvhdBody, binary.BigEndian, timescale)
	_ = binary.Write(mvhdBody, binary.BigEndian, duration)
	_ = binary.Write(mvhdBody, binary.BigEndian, int32(0x00010000)) // rate
	_ = binary.Write(mvhdBody, binary.BigEndian, int16(0x0100))     // volume
	_ = binary.Write(mvhdBody, binary.BigEndian, int16(0))          // reserved
	mvhdBody.Write(make([]byte, 8))                                 // reserved2 [2]uint32
	mvhdBody.Write(make([]byte, 36))                                // matrix [9]int32
	mvhdBody.Write(make([]byte, 24))                                // pre_defined [6]int32
	_ = binary.Write(mvhdBody, binary.BigEndian, uint32(1))         // next track id

	mvhd := &bytes.Buffer{}
	_ = binary.Write(mvhd, binary.BigEndian, uint32(8+mvhdBody.Len()))
	mvhd.WriteString("mvhd")
	mvhd.Write(mvhdBody.Bytes())

	moov := &bytes.Buffer{}
	_ = binary.Write(moov, binary.BigEndian, uint32(8+mvhd.Len()))
	moov.WriteString("moov")
	moov.Write(mvhd.Bytes())

	return moov.Bytes()
}

// TestGetM4ADuration_Valid feeds a hand-built moov/mvhd box (timescale 1000,
// duration 2000 ⇒ 2.0s) and asserts getM4ADuration's Duration/Timescale math.
// This is the parse-success branch (0% before) that malformed input can't hit.
func TestGetM4ADuration_Valid(t *testing.T) {
	mp4Bytes := buildMP4(1000, 2000)
	d, err := GetAudioDuration(context.Background(), bytes.NewReader(mp4Bytes), ".m4a")
	if err != nil {
		t.Fatalf("valid m4a probe error: %v", err)
	}
	if d < 1.99 || d > 2.01 {
		t.Errorf("expected ~2.0s duration, got %f", d)
	}

	// The .mp4 dispatch alias reaches the same parser.
	d2, err := GetAudioDuration(context.Background(), bytes.NewReader(buildMP4(48000, 24000)), ".mp4")
	if err != nil {
		t.Fatalf("valid mp4 probe error: %v", err)
	}
	if d2 < 0.49 || d2 > 0.51 {
		t.Errorf("expected ~0.5s duration, got %f", d2)
	}
}

// TestGetM4ADuration_Malformed guarantees a corrupt MP4 upload surfaces an error
// (billing degrades safely) rather than panicking.
func TestGetM4ADuration_Malformed(t *testing.T) {
	// Truncated moov box header claims a large size but has no body.
	garbage := []byte{0x00, 0x00, 0x00, 0x64, 'm', 'o', 'o', 'v', 0x00, 0x00}
	if _, err := GetAudioDuration(context.Background(), bytes.NewReader(garbage), ".m4a"); err == nil {
		t.Error("malformed m4a should error, not panic")
	}
}

// TestGetFLACDuration_Truncated guarantees a FLAC signature followed by a
// truncated STREAMINFO body surfaces an error rather than panicking.
func TestGetFLACDuration_Truncated(t *testing.T) {
	// Valid signature + block header claiming 34 bytes, but body cut short.
	trunc := append([]byte("fLaC"), 0x80, 0x00, 0x00, 34, 0x10, 0x00)
	if _, err := GetAudioDuration(context.Background(), bytes.NewReader(trunc), ".flac"); err == nil {
		t.Error("truncated flac should error, not panic")
	}
}
