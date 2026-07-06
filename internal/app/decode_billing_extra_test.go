package app

// decode_billing_extra_test.go — coverage for the pure image/file decoders and
// the DB-backed subscription/usage amount queries.

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

// ─── file_decoder.go: GetMimeTypeByExtension ──────────────────────────────

func TestGetMimeTypeByExtension(t *testing.T) {
	cases := map[string]string{
		"txt":      "text/plain",
		"md":       "text/plain",
		"markdown": "text/plain",
		"csv":      "text/plain",
		"xml":      "text/plain",
		"html":     "text/plain",
		"htm":      "text/plain",
		"MD":       "text/plain", // case-insensitive
		"json":     "text/plain",
		"jpg":      "image/jpeg",
		"jpeg":     "image/jpeg",
		"png":      "image/png",
		"gif":      "image/gif",
		"mp3":      "audio/mp3",
		"wav":      "audio/wav",
		"mpeg":     "audio/mpeg",
		"mp4":      "video/mp4",
		"wmv":      "video/wmv",
		"flv":      "video/flv",
		"mov":      "video/mov",
		"mpg":      "video/mpg",
		"avi":      "video/avi",
		"mpegps":   "video/mpegps",
		"pdf":      "application/pdf",
		"unknown":  "application/octet-stream",
		"":         "application/octet-stream",
	}
	for ext, want := range cases {
		if got := GetMimeTypeByExtension(ext); got != want {
			t.Errorf("GetMimeTypeByExtension(%q) = %q, want %q", ext, got, want)
		}
	}
}

// ─── image.go: base64 decoders ────────────────────────────────────────────

// tinyPNGBase64 builds a valid 2x1 PNG and returns its std-base64 encoding.
func tinyPNGBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestDecodeBase64ImageData_ValidPNG(t *testing.T) {
	raw := tinyPNGBase64(t)
	// With a data-URL prefix, the prefix must be stripped before decoding.
	cfg, format, clean, err := DecodeBase64ImageData("data:image/png;base64," + raw)
	if err != nil {
		t.Fatalf("DecodeBase64ImageData: %v", err)
	}
	if format != "png" {
		t.Errorf("format = %q, want png", format)
	}
	if cfg.Width != 2 || cfg.Height != 1 {
		t.Errorf("config = %dx%d, want 2x1", cfg.Width, cfg.Height)
	}
	if clean != raw {
		t.Errorf("returned base64 should be the prefix-stripped payload")
	}
}

func TestDecodeBase64ImageData_EmptyErrors(t *testing.T) {
	if _, _, _, err := DecodeBase64ImageData(""); err == nil {
		t.Fatal("expected error for empty base64 string")
	}
}

func TestDecodeBase64ImageData_InvalidBase64Errors(t *testing.T) {
	if _, _, _, err := DecodeBase64ImageData("!!!not-base64!!!"); err == nil {
		t.Fatal("expected error for undecodable base64")
	}
}

func TestDecodeBase64ImageData_ValidBase64NonImage(t *testing.T) {
	// Valid base64 but not a decodable image => base64 decodes, then both
	// image.DecodeConfig and the webp fallback in getImageConfig fail, so the
	// call returns an error (exercises the webp-fallback error path).
	notAnImage := base64.StdEncoding.EncodeToString([]byte("this is plain text, not an image"))
	if _, _, _, err := DecodeBase64ImageData(notAnImage); err == nil {
		t.Fatal("expected error decoding non-image bytes")
	}
}

func TestDecodeBase64FileData_WithMimePrefix(t *testing.T) {
	raw := tinyPNGBase64(t)
	mime, payload, err := DecodeBase64FileData("data:image/png;base64," + raw)
	if err != nil {
		t.Fatalf("DecodeBase64FileData: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("mime = %q, want image/png", mime)
	}
	if payload != raw {
		t.Errorf("payload should be the raw base64 body")
	}
}

func TestDecodeBase64FileData_HeaderWithoutSemicolon(t *testing.T) {
	raw := tinyPNGBase64(t)
	// A "data:...,<b64>" header with a comma but no ';' takes the semicolon-less
	// branch, which falls back to image decoding and yields "image/<format>".
	mime, _, err := DecodeBase64FileData("data:image/png," + raw)
	if err != nil {
		t.Fatalf("DecodeBase64FileData: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("mime = %q, want image/png (semicolon-less header path)", mime)
	}
}

func TestDecodeBase64FileData_NoPrefixFallsBackToImage(t *testing.T) {
	raw := tinyPNGBase64(t)
	// No comma => treated as bare image base64; mime becomes "image/<format>".
	mime, _, err := DecodeBase64FileData(raw)
	if err != nil {
		t.Fatalf("DecodeBase64FileData no-prefix: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("mime = %q, want image/png", mime)
	}
}

// ─── billing_service.go: subscription/usage amounts ───────────────────────

func TestGetSubscriptionQuotaInfo_UserLevel(t *testing.T) {
	db := setupServiceTestDB(t)
	userId := seedTestUser(t, db, 500_000)
	// Give the user some used quota so used/total differ.
	repo.UpdateUserUsedQuotaAndRequestCount(userId, 100_000)

	info, err := GetSubscriptionQuotaInfo(userId, 0, false)
	if err != nil {
		t.Fatalf("GetSubscriptionQuotaInfo: %v", err)
	}
	// Default display type is USD: amount = quota / QuotaPerUnit.
	wantRemaining := float64(500_000) / common.QuotaPerUnit
	if info.RemainingAmount != wantRemaining {
		t.Errorf("RemainingAmount = %v, want %v", info.RemainingAmount, wantRemaining)
	}
	wantUsed := float64(100_000) / common.QuotaPerUnit
	if info.UsedAmount != wantUsed {
		t.Errorf("UsedAmount = %v, want %v", info.UsedAmount, wantUsed)
	}
	wantTotal := float64(600_000) / common.QuotaPerUnit
	if info.TotalAmount != wantTotal {
		t.Errorf("TotalAmount = %v, want %v", info.TotalAmount, wantTotal)
	}
}

func TestGetSubscriptionQuotaInfo_TokenLevelUnlimited(t *testing.T) {
	db := setupServiceTestDB(t)
	userId := seedTestUser(t, db, 1_000)
	_, tokenId := seedTestToken(t, db, userId, 0, true) // unlimited token

	info, err := GetSubscriptionQuotaInfo(userId, tokenId, true)
	if err != nil {
		t.Fatalf("GetSubscriptionQuotaInfo token: %v", err)
	}
	if !info.UnlimitedQuota {
		t.Error("UnlimitedQuota = false, want true")
	}
	if info.TotalAmount != 100000000 {
		t.Errorf("unlimited TotalAmount = %v, want 100000000", info.TotalAmount)
	}
}

func TestGetUsageAmount_UserLevel(t *testing.T) {
	db := setupServiceTestDB(t)
	userId := seedTestUser(t, db, 100_000)
	repo.UpdateUserUsedQuotaAndRequestCount(userId, 250_000)

	got, err := GetUsageAmount(userId, 0, false)
	if err != nil {
		t.Fatalf("GetUsageAmount: %v", err)
	}
	// usage = CalculateDisplayAmount(usedQuota) * 100
	want := (float64(250_000) / common.QuotaPerUnit) * 100
	if got != want {
		t.Errorf("GetUsageAmount = %v, want %v", got, want)
	}
}

func TestGetUsageAmount_TokenLevel(t *testing.T) {
	db := setupServiceTestDB(t)
	userId := seedTestUser(t, db, 100_000)
	_, tokenId := seedTestToken(t, db, userId, 10_000, false)
	// Bump the token used quota directly.
	if err := db.Model(&repo.Token{}).Where("id = ?", tokenId).Update("used_quota", 30_000).Error; err != nil {
		t.Fatalf("set token used: %v", err)
	}

	got, err := GetUsageAmount(userId, tokenId, true)
	if err != nil {
		t.Fatalf("GetUsageAmount token: %v", err)
	}
	want := (float64(30_000) / common.QuotaPerUnit) * 100
	if got != want {
		t.Errorf("GetUsageAmount token = %v, want %v", got, want)
	}
}
