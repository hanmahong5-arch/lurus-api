package common

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"testing"
)

// f64ToExtended80 encodes a float64 as an 80-bit IEEE-754 extended-precision
// value (big-endian), the sample-rate representation AIFF COMM chunks use.
func f64ToExtended80(f float64) [10]byte {
	var b [10]byte
	if f == 0 {
		return b
	}
	sign := 0
	if f < 0 {
		sign = 1
		f = -f
	}
	mant, exp := math.Frexp(f) // f = mant * 2^exp, mant in [0.5,1)
	exp--
	mant *= 2 // mant now in [1,2)
	e := exp + 16383
	b[0] = byte(sign<<7 | (e>>8)&0x7f)
	b[1] = byte(e & 0xff)
	binary.BigEndian.PutUint64(b[2:], uint64(math.Ldexp(mant, 63)))
	return b
}

// buildAIFF hand-assembles a minimal, valid AIFF file: FORM/AIFF + COMM + SSND.
func buildAIFF(numChannels uint16, numSampleFrames uint32, sampleSize uint16, sampleRate float64) []byte {
	comm := &bytes.Buffer{}
	_ = binary.Write(comm, binary.BigEndian, numChannels)
	_ = binary.Write(comm, binary.BigEndian, numSampleFrames)
	_ = binary.Write(comm, binary.BigEndian, sampleSize)
	sr := f64ToExtended80(sampleRate)
	comm.Write(sr[:])

	dataLen := int(numSampleFrames) * int(numChannels) * int(sampleSize) / 8
	ssnd := &bytes.Buffer{}
	_ = binary.Write(ssnd, binary.BigEndian, uint32(0)) // offset
	_ = binary.Write(ssnd, binary.BigEndian, uint32(0)) // blockSize
	ssnd.Write(make([]byte, dataLen))

	body := &bytes.Buffer{}
	body.WriteString("AIFF")
	body.WriteString("COMM")
	_ = binary.Write(body, binary.BigEndian, uint32(comm.Len()))
	body.Write(comm.Bytes())
	body.WriteString("SSND")
	_ = binary.Write(body, binary.BigEndian, uint32(ssnd.Len()))
	body.Write(ssnd.Bytes())

	out := &bytes.Buffer{}
	out.WriteString("FORM")
	_ = binary.Write(out, binary.BigEndian, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// adtsFrame builds one AAC-LC ADTS frame (7-byte header, no CRC) for the given
// sampling-frequency index and channel config, padded with a zero payload.
func adtsFrame(freqIdx, channels, payloadLen int) []byte {
	frameLen := 7 + payloadLen
	h := make([]byte, 7)
	h[0] = 0xFF
	h[1] = 0xF1 // syncword tail + MPEG-4 + layer 0 + protection-absent
	h[2] = byte(0x40 | ((freqIdx & 0xF) << 2) | ((channels >> 2) & 1))
	h[3] = byte(((channels & 3) << 6) | ((frameLen >> 11) & 3))
	h[4] = byte((frameLen >> 3) & 0xFF)
	h[5] = byte(((frameLen & 7) << 5) | 0x1F)
	h[6] = 0xFC
	return append(h, make([]byte, payloadLen)...)
}

// TestGetAACDuration_Valid feeds a hand-built ADTS AAC stream (16 kHz mono, 16
// frames) and asserts the frame-count × 1024-samples duration math.
func TestGetAACDuration_Valid(t *testing.T) {
	const freqIdx = 8 // 16000 Hz
	const frames = 16
	var stream []byte
	for i := 0; i < frames; i++ {
		stream = append(stream, adtsFrame(freqIdx, 1, 8)...)
	}
	d, err := GetAudioDuration(context.Background(), bytes.NewReader(stream), ".aac")
	if err != nil {
		t.Fatalf("valid aac parse error: %v", err)
	}
	// 16 frames × 1024 samples / 16000 Hz = 1.024s.
	want := float64(frames*1024) / 16000.0
	if d < want-0.05 || d > want+0.05 {
		t.Errorf("aac duration = %f, want ~%f", d, want)
	}
}

// TestGetWAVDuration_ZeroDataSize builds a WAV whose data chunk declares size 0
// (a real-world quirk of some streamed encoders) and asserts getWAVDuration's
// fallback path infers the PCM size from the actual file length.
func TestGetWAVDuration_ZeroDataSize(t *testing.T) {
	const sampleRate = 8000
	const bitsPerSample = 16
	const numChannels = 1
	dataBytes := sampleRate * (bitsPerSample / 8) * numChannels // 1 second worth

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataBytes))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	byteRate := sampleRate * numChannels * (bitsPerSample / 8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels*bitsPerSample/8))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // declared size 0 ⇒ fallback
	buf.Write(make([]byte, dataBytes))                    // real payload still present

	d, err := GetAudioDuration(context.Background(), bytes.NewReader(buf.Bytes()), ".wav")
	if err != nil {
		t.Fatalf("zero-size wav parse error: %v", err)
	}
	if d < 0.9 || d > 1.1 {
		t.Errorf("fallback duration = %f, want ~1.0s", d)
	}
}

// TestGetAIFFDuration_Valid feeds a hand-built 8 kHz mono 16-bit AIFF of exactly
// one second (8000 sample frames) and asserts the decoded duration.
func TestGetAIFFDuration_Valid(t *testing.T) {
	aiffBytes := buildAIFF(1, 8000, 16, 8000)
	d, err := GetAudioDuration(context.Background(), bytes.NewReader(aiffBytes), ".aiff")
	if err != nil {
		t.Fatalf("valid aiff parse error: %v", err)
	}
	if d < 0.99 || d > 1.01 {
		t.Errorf("expected ~1.0s duration, got %f", d)
	}

	// A two-second 44.1 kHz stereo clip lands on the expected duration too,
	// proving the sample-rate/frame math (not a hard-coded 1.0) drives the result.
	aiff2 := buildAIFF(2, 88200, 16, 44100)
	d2, err := GetAudioDuration(context.Background(), bytes.NewReader(aiff2), ".aif")
	if err != nil {
		t.Fatalf("valid aiff(2s) parse error: %v", err)
	}
	if d2 < 1.99 || d2 > 2.01 {
		t.Errorf("expected ~2.0s duration, got %f", d2)
	}
}
