package app

// token_counter_audio_gap_test.go — drives EstimateRequestToken's audio
// transcription/translation branch: a multipart upload is parsed, each file's
// duration is measured, and 1000 tokens/minute are billed. Covers both the
// unsupported-format error arm and the successful WAV-duration accounting arm.

import (
	"bytes"
	"encoding/binary"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	constant2 "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

func audioRequest(t *testing.T, filename string, body []byte) *http.Request {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// minimalWAV builds a canonical 8kHz/mono/8-bit PCM WAV of `samples` bytes of
// audio data (1 byte/sample), i.e. samples/8000 seconds long.
func minimalWAV(samples int) []byte {
	const sampleRate = 8000
	buf := &bytes.Buffer{}
	data := make([]byte, samples)
	dataLen := uint32(len(data))
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(36+dataLen)) // file size - 8
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))         // fmt chunk size
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))          // PCM
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))          // mono
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate)) // sample rate
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate)) // byte rate
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))          // block align
	_ = binary.Write(buf, binary.LittleEndian, uint16(8))          // bits/sample
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataLen)
	buf.Write(data)
	return buf.Bytes()
}

func TestEstimateRequestToken_AudioTranscriptionUnsupportedFormat(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prev })
	prevBody := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 10
	t.Cleanup(func() { constant.MaxRequestBodyMB = prevBody })

	c := createTestGinContext()
	c.Request = audioRequest(t, "clip.xyz", []byte("not real audio"))

	info := &relaycommon.RelayInfo{RelayMode: constant2.RelayModeAudioTranscription}
	if _, err := EstimateRequestToken(c, &types.TokenCountMeta{}, info); err == nil {
		t.Fatal("expected an error for an unsupported audio format")
	}
}

func TestEstimateRequestToken_AudioTranscriptionWAVDuration(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prev })
	prevBody := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 10
	t.Cleanup(func() { constant.MaxRequestBodyMB = prevBody })

	// 16000 samples @ 8000Hz = 2 seconds => ceil(2)/60*1000 ≈ 33 tokens.
	c := createTestGinContext()
	c.Request = audioRequest(t, "clip.wav", minimalWAV(16000))

	info := &relaycommon.RelayInfo{RelayMode: constant2.RelayModeAudioTranscription}
	got, err := EstimateRequestToken(c, &types.TokenCountMeta{}, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	if got <= 0 {
		t.Errorf("audio token count = %d, want > 0 (2s of audio billed per minute)", got)
	}
}
