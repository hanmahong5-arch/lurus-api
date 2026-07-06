package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/model_setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// TestGeminiHelper_ParamOverride covers the ParamOverride application branch of
// GeminiHelper (native channel).
func TestGeminiHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-ovr", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiGenerateBody))
	}))
	defer srv.Close()

	req := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{}, req, u.Id)
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"generationConfig": map[string]any{"temperature": 0.25}})

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("0.25")) {
		t.Errorf("param override not applied to gemini body: %s", sawBody)
	}
}

// TestGeminiHelper_NoThinkingBudgetAdapter drives the no-thinking adapter branch:
// with the Gemini thinking adapter enabled and a request that explicitly sets a
// zero thinking budget, isNoThinkingRequest is true and the handler attempts to
// resolve a "-nothinking" priced model variant before dispatch.
func TestGeminiHelper_NoThinkingBudgetAdapter(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	gs := model_setting.GetGeminiSettings()
	prev := gs.ThinkingAdapterEnabled
	gs.ThinkingAdapterEnabled = true
	defer func() { gs.ThinkingAdapterEnabled = prev }()

	u := &repo.User{Username: "gemini-nothink", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiGenerateBody))
	}))
	defer srv.Close()

	zeroBudget := 0
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}},
	}
	req.GenerationConfig.ThinkingConfig = &dto.GeminiThinkingConfig{ThinkingBudget: &zeroBudget}

	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{}, req, u.Id)

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper no-thinking adapter returned error: %v", err.Error())
	}
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (post-consume ran)", refreshed.RequestCount)
	}
}

// TestRelayMidjourneyImage_SSRFRejected covers the SSRF-protection rejection
// branch of RelayMidjourneyImage: with protection enabled and a private-IP image
// URL, the fetch is rejected with 403 before any HTTP GET.
func TestRelayMidjourneyImage_SSRFRejected(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	fs := system_setting.GetFetchSetting()
	prevSSRF, prevPriv := fs.EnableSSRFProtection, fs.AllowPrivateIp
	fs.EnableSSRFProtection = true
	fs.AllowPrivateIp = false
	defer func() { fs.EnableSSRFProtection = prevSSRF; fs.AllowPrivateIp = prevPriv }()

	if err := repo.MjInsert(&repo.Midjourney{MjId: "ssrf-1", UserId: 4, ImageUrl: "http://127.0.0.1:9/secret.png"}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}

	c, rec := mjGinContext(t, nil)
	c.Params = gin.Params{{Key: "id", Value: "ssrf-1"}}
	RelayMidjourneyImage(c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (SSRF rejection)", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("SSRF")) {
		t.Errorf("expected SSRF rejection body, got: %s", rec.Body.String())
	}
}

// TestRelayMidjourneyImage_DefaultsContentType covers the empty-Content-Type
// default branch: an upstream image response with no Content-Type header is
// streamed with a default image/jpeg content type.
func TestRelayMidjourneyImage_DefaultsContentType(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	fs := system_setting.GetFetchSetting()
	prevSSRF, prevPriv := fs.EnableSSRFProtection, fs.AllowPrivateIp
	fs.EnableSSRFProtection = false
	fs.AllowPrivateIp = true
	defer func() { fs.EnableSSRFProtection = prevSSRF; fs.AllowPrivateIp = prevPriv }()

	imgBytes := []byte("\xff\xd8\xfffake")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately omit Content-Type so the handler applies its default.
		hdr := w.Header()
		hdr["Content-Type"] = nil
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(imgBytes)
	}))
	defer srv.Close()

	if err := repo.MjInsert(&repo.Midjourney{MjId: "ct-1", UserId: 4, ImageUrl: srv.URL}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}

	c, rec := mjGinContext(t, nil)
	c.Params = gin.Params{{Key: "id", Value: "ct-1"}}
	RelayMidjourneyImage(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want default image/jpeg", ct)
	}
}
