package app

// final_extra_test.go — remaining reachable branches: EstimateRequestToken with
// a data-URL media file, the token-key generator, release download counting,
// the notify-limit Redis limit-reached branch, and reportQuotaThreshold's
// full nats-enabled early guards.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

// httptestNewImageServer returns a server that answers any path with an
// image/png Content-Type header (body content is irrelevant for type sniffing).
func httptestNewImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pngdata"))
	}))
}

// ─── token_counter.go: EstimateRequestToken with a data-URL file ──────────

func TestEstimateRequestToken_DataURLAudioFile(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	// Non-OpenAI model avoids the image-decode path; an audio data-URL exercises
	// the data: prefix parse + the audio file-type token add (+256).
	c.Set(string(constant.ContextKeyOriginalModel), "claude-3-5-sonnet")

	meta := &types.TokenCountMeta{
		CombineText: "describe this clip",
		Files: []*types.FileMeta{
			{OriginData: "data:audio/mp3;base64,QUJD"},
		},
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	// Must include the +256 audio contribution plus the text tokens.
	if got < 256 {
		t.Errorf("token estimate = %d, want >= 256 (audio file add)", got)
	}
}

func TestEstimateRequestToken_ImageFileOpenAIModel(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	// OpenAI model routes the image file through getImageToken. GetMediaToken is
	// false by default, so getImageToken returns 3*baseTokens without decoding.
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-4o")

	meta := &types.TokenCountMeta{
		CombineText: "what's in this image",
		Files: []*types.FileMeta{
			{OriginData: "data:image/png;base64,QUJD"},
		},
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	// 3 * 85 (base tokens for the 4o family) plus the text tokens.
	if got < 255 {
		t.Errorf("token estimate = %d, want >= 255 (image token floor)", got)
	}
}

func TestEstimateRequestToken_HTTPFileFetchesType(t *testing.T) {
	prevCount := constant.CountToken
	prevMedia := constant.GetMediaToken
	prevMediaNS := constant.GetMediaTokenNotStream
	constant.CountToken = true
	constant.GetMediaToken = true
	constant.GetMediaTokenNotStream = true
	defer func() {
		constant.CountToken = prevCount
		constant.GetMediaToken = prevMedia
		constant.GetMediaTokenNotStream = prevMediaNS
	}()

	allowLocalFetch(t)
	srv := httptestNewImageServer(t)
	defer srv.Close()

	c := createTestGinContext()
	c.Set(string(constant.ContextKeyOriginalModel), "claude-3-5-sonnet") // non-openai => no image decode

	meta := &types.TokenCountMeta{
		CombineText: "look at this",
		Files: []*types.FileMeta{
			{OriginData: srv.URL + "/pic.png"}, // http url => triggers GetFileTypeFromUrl
		},
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	// Non-OpenAI model + resolved image file => +520 image token block.
	if got < 520 {
		t.Errorf("token estimate = %d, want >= 520 (fetched image file)", got)
	}
}

func TestEstimateRequestToken_DataURLVideoAndFile(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	c.Set(string(constant.ContextKeyOriginalModel), "claude-3-5-sonnet")

	meta := &types.TokenCountMeta{
		CombineText: "process these",
		Files: []*types.FileMeta{
			{OriginData: "data:video/mp4;base64,QUJD"},       // video => +8192
			{OriginData: "data:application/pdf;base64,QUJD"}, // generic file => +4096
		},
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	// video (4096*2) + file (4096) contributions plus text.
	if got < 8192+4096 {
		t.Errorf("estimate = %d, want >= %d (video+file)", got, 8192+4096)
	}
}

func TestEstimateRequestToken_GeminiFormatSkipsFetch(t *testing.T) {
	prevCount := constant.CountToken
	prevMedia := constant.GetMediaToken
	constant.CountToken = true
	constant.GetMediaToken = true // even so, Gemini format sets shouldFetchFiles=false
	defer func() {
		constant.CountToken = prevCount
		constant.GetMediaToken = prevMedia
	}()

	c := createTestGinContext()
	c.Set(string(constant.ContextKeyOriginalModel), "gemini-1.5-pro")

	// An http file URL that would fail if fetched — but Gemini format must NOT
	// fetch it, so no error and the file falls to the default token bucket.
	meta := &types.TokenCountMeta{
		CombineText: "look",
		Files: []*types.FileMeta{
			{OriginData: "http://192.0.2.1/never-fetched.png"}, // TEST-NET-1, unroutable
		},
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatGemini}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken gemini: %v", err)
	}
	if got <= 0 {
		t.Errorf("estimate = %d, want > 0", got)
	}
}

// ─── token_service.go: GenerateTokenKey ───────────────────────────────────

func TestGenerateTokenKey(t *testing.T) {
	k1, err := GenerateTokenKey()
	if err != nil {
		t.Fatalf("GenerateTokenKey: %v", err)
	}
	if k1 == "" {
		t.Fatal("generated key is empty")
	}
	k2, _ := GenerateTokenKey()
	if k1 == k2 {
		t.Error("two generated keys must differ")
	}
}

// ─── release_service.go: HandleDownload increments the count ──────────────

func TestHandleDownload_IncrementsCount(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)

	rel := entity.Release{ProductId: "switch", Version: "1.0.0", Title: "R", IsPublished: true}
	if err := db.Create(&rel).Error; err != nil {
		t.Fatalf("seed release: %v", err)
	}
	art := entity.ReleaseArtifact{
		ReleaseId:      rel.Id,
		Platform:       "linux",
		Arch:           "amd64",
		Filename:       "switch-linux-amd64",
		FileSize:       10,
		StoragePath:    "tools/switch/1.0.0/switch-linux-amd64",
		ChecksumSha256: "deadbeef",
	}
	if err := db.Create(&art).Error; err != nil {
		t.Fatalf("seed artifact: %v", err)
	}

	if err := svc.HandleDownload(context.Background(), art.Id, "1.2.3.4", "agent", "ref"); err != nil {
		t.Fatalf("HandleDownload: %v", err)
	}

	var got entity.ReleaseArtifact
	if err := db.First(&got, art.Id).Error; err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if got.DownloadCount != 1 {
		t.Errorf("download_count = %d, want 1", got.DownloadCount)
	}
	// Let the async download-log goroutine finish before teardown.
	time.Sleep(50 * time.Millisecond)
}

// ─── notify-limit.go: Redis limit-reached branch ──────────────────────────

func TestCheckNotificationLimit_RedisLimitReached(t *testing.T) {
	client := withRedis(t)
	userID := 750000 + int(time.Now().UnixNano()%100000)
	notifyType := "quota_exceed"
	key := fmt.Sprintf("notify_limit:%d:%s:%s", userID, notifyType, time.Now().Format("2006010215"))
	ctx := context.Background()
	t.Cleanup(func() { client.Del(ctx, key) })

	prev := constant.NotifyLimitCount
	constant.NotifyLimitCount = 2
	t.Cleanup(func() { constant.NotifyLimitCount = prev })

	// Pre-seed the counter at the limit so the next check is rejected.
	client.Set(ctx, key, "2", time.Hour)

	ok, err := CheckNotificationLimit(ctx, userID, notifyType)
	if err != nil {
		t.Fatalf("CheckNotificationLimit: %v", err)
	}
	if ok {
		t.Error("expected notification to be rejected at the limit")
	}
}

// ─── quota.go: reportQuotaThreshold nats-enabled guards ───────────────────

func TestReportQuotaThreshold_NatsEnabledMissingAccountGuard(t *testing.T) {
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")
	// nats.Get() is nil (no publisher initialized), so the function must return
	// at the nil-publisher guard without touching the DB.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("reportQuotaThreshold panicked: %v", r)
		}
	}()
	reportQuotaThreshold(context.Background(), &relaycommon.RelayInfo{
		IdentityAccountID: 10,
		UserId:            20,
	}, 500)
}

var _ = repo.Token{}
var _ = common.GetTimestamp
