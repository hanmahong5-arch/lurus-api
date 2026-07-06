package app

// release_service_extra_test.go — coverage for the release/download service.
// The MinIO-backed presign paths need object storage, so these focus on the
// storage-agnostic surface: pure filename/version parsing, the npm/manifest
// builder (npm-only when storage is unconfigured), and the DB-backed
// list/get/changelog queries against a seeded SQLite tier.

import (
	"context"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"gorm.io/gorm"
)

// ─── pure helpers ─────────────────────────────────────────────────────────

func TestParsePlatformFromFilename(t *testing.T) {
	cases := []struct {
		tool, filename string
		wantOS, wantAr string
	}{
		{"switch", "switch-windows-amd64.exe", "windows", "amd64"},
		{"switch", "switch-linux-arm64.tar.gz", "linux", "arm64"},
		{"switch", "switch-darwin-x86_64.dmg", "darwin", "amd64"},  // x86_64 => amd64
		{"switch", "switch-darwin-aarch64.zip", "darwin", "arm64"}, // aarch64 => arm64
		{"switch", "wrong-prefix-linux-amd64", "", ""},             // prefix mismatch
		{"switch", "switch-solaris-amd64", "", ""},                 // invalid OS
		{"switch", "switch-linux-mips", "", ""},                    // invalid arch
		{"switch", "switch-linux", "", ""},                         // missing arch segment
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			os, arch := parsePlatformFromFilename(tc.tool, tc.filename)
			if os != tc.wantOS || arch != tc.wantAr {
				t.Errorf("parse(%q) = (%q,%q), want (%q,%q)", tc.filename, os, arch, tc.wantOS, tc.wantAr)
			}
		})
	}
}

func TestLatestSemverKey(t *testing.T) {
	versions := map[string][]storageObject{
		"1.0.0":  nil,
		"1.2.0":  nil,
		"1.10.0": nil, // must beat 1.2.0 by semver, not lexicographically
		"0.9.9":  nil,
	}
	if got := latestSemverKey(versions); got != "1.10.0" {
		t.Errorf("latestSemverKey = %q, want 1.10.0", got)
	}
	if got := latestSemverKey(map[string][]storageObject{}); got != "" {
		t.Errorf("empty versions => %q, want empty", got)
	}
}

func TestCompareVersions_Extra(t *testing.T) {
	cases := []struct {
		v1, v2 string
		want   int
	}{
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.2.3", "1.2.3", 0},
		{"v1.0.0", "1.0.0", 0}, // v-prefix normalization
	}
	for _, tc := range cases {
		if got := compareVersions(tc.v1, tc.v2); got != tc.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", tc.v1, tc.v2, got, tc.want)
		}
	}
}

func TestExtractCountryFromIP(t *testing.T) {
	// Current implementation returns "" for any IP (geo lookup not wired), but
	// invalid input must also yield "" without panicking.
	if got := extractCountryFromIP("8.8.8.8"); got != "" {
		t.Errorf("valid IP => %q, want empty (geo unwired)", got)
	}
	if got := extractCountryFromIP("not-an-ip"); got != "" {
		t.Errorf("invalid IP => %q, want empty", got)
	}
}

func TestNpmTools(t *testing.T) {
	tools := npmTools()
	for _, name := range []string{"claude", "codex", "gemini", "openclaw"} {
		e, ok := tools[name]
		if !ok {
			t.Errorf("npmTools missing %q", name)
			continue
		}
		if e.Type != "npm" || e.NpmPackage == "" || e.LatestVersion == "" {
			t.Errorf("npm tool %q incomplete: %+v", name, e)
		}
	}
}

// ─── service wiring (no MinIO configured) ─────────────────────────────────

func newReleaseSvc(t *testing.T, db *gorm.DB) *ReleaseService {
	t.Helper()
	if err := db.AutoMigrate(&entity.Release{}, &entity.ReleaseArtifact{}); err != nil {
		t.Fatalf("migrate release tables: %v", err)
	}
	return NewReleaseService(repo.NewReleaseRepository(db))
}

func TestNewReleaseService_NoStorage(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	if svc.IsStorageConfigured() {
		t.Error("IsStorageConfigured should be false without MINIO_ENDPOINT")
	}
}

func TestBuildToolManifest_NpmOnlyAndCached(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)

	resp, err := svc.BuildToolManifest(context.Background())
	if err != nil {
		t.Fatalf("BuildToolManifest: %v", err)
	}
	// No MinIO => only the four npm tools.
	if len(resp.Tools) != 4 {
		t.Errorf("manifest tool count = %d, want 4 (npm-only)", len(resp.Tools))
	}
	if _, ok := resp.Tools["claude"]; !ok {
		t.Error("manifest missing claude npm tool")
	}

	// Second call must hit the cache (fast path) and return the same GeneratedAt.
	resp2, err := svc.BuildToolManifest(context.Background())
	if err != nil {
		t.Fatalf("BuildToolManifest cached: %v", err)
	}
	if resp2.GeneratedAt != resp.GeneratedAt {
		t.Errorf("cached manifest GeneratedAt = %q, want same as first %q", resp2.GeneratedAt, resp.GeneratedAt)
	}
}

// ─── DB-backed queries ────────────────────────────────────────────────────

func seedRelease(t *testing.T, db *gorm.DB, productId, version string, published bool, publishedAt time.Time) int64 {
	t.Helper()
	pa := publishedAt
	rel := entity.Release{
		ProductId:   productId,
		Version:     version,
		Title:       "Release " + version,
		ChangelogMd: "## " + version + "\n- changes",
		ReleaseType: "stable",
		IsPublished: published,
		PublishedAt: &pa,
	}
	if err := db.Create(&rel).Error; err != nil {
		t.Fatalf("seed release: %v", err)
	}
	return rel.Id
}

func TestListReleases_FiltersAndPaginates(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	base := time.Now()
	seedRelease(t, db, "switch", "1.0.0", true, base.Add(-2*time.Hour))
	seedRelease(t, db, "switch", "1.1.0", true, base.Add(-1*time.Hour))
	seedRelease(t, db, "creator", "0.1.0", true, base)
	seedRelease(t, db, "switch", "1.2.0", false, base) // unpublished, excluded

	resp, err := svc.ListReleases(context.Background(), repo.ListReleasesParams{
		ProductId: "switch",
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("published switch releases = %d, want 2", resp.Total)
	}
	// Ordered by published_at DESC => newest first.
	if len(resp.Releases) > 0 && resp.Releases[0].Version != "1.1.0" {
		t.Errorf("first release = %q, want 1.1.0 (newest)", resp.Releases[0].Version)
	}
}

func TestListReleases_IncludePrereleaseAndClampPageSize(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	base := time.Now()
	seedRelease(t, db, "switch", "1.0.0", true, base)
	// A published prerelease.
	pre := entity.Release{ProductId: "switch", Version: "2.0.0-rc1", Title: "rc", IsPublished: true, IsPrerelease: true, PublishedAt: &base}
	if err := db.Create(&pre).Error; err != nil {
		t.Fatalf("seed prerelease: %v", err)
	}

	// IncludePrerelease=true + an out-of-range PageSize (clamped to 20).
	resp, err := svc.ListReleases(context.Background(), repo.ListReleasesParams{
		ProductId:         "switch",
		IncludePrerelease: true,
		Page:              0,   // clamped to 1
		PageSize:          500, // clamped to 20
	})
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2 (stable + prerelease)", resp.Total)
	}
	// Both rows are returned despite the oversized PageSize (repo clamps to 20).
	if len(resp.Releases) != 2 {
		t.Errorf("returned %d releases, want 2 (clamped page size still fits both)", len(resp.Releases))
	}
}

func TestGetLatestRelease_HasUpdate(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	base := time.Now()
	seedRelease(t, db, "switch", "1.0.0", true, base.Add(-1*time.Hour))
	seedRelease(t, db, "switch", "2.0.0", true, base)

	// Current version older than latest => HasUpdate true.
	resp, err := svc.GetLatestRelease(context.Background(), "switch", "1.0.0")
	if err != nil {
		t.Fatalf("GetLatestRelease: %v", err)
	}
	if resp.Release == nil || resp.Release.Version != "2.0.0" {
		t.Fatalf("latest = %+v, want 2.0.0", resp.Release)
	}
	if !resp.HasUpdate {
		t.Error("HasUpdate = false, want true (1.0.0 < 2.0.0)")
	}

	// Current version equals latest => no update.
	resp2, _ := svc.GetLatestRelease(context.Background(), "switch", "2.0.0")
	if resp2.HasUpdate {
		t.Error("HasUpdate = true for equal version, want false")
	}
}

func TestGetLatestRelease_NoneReturnsNilNoUpdate(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	resp, err := svc.GetLatestRelease(context.Background(), "nonexistent", "1.0.0")
	if err != nil {
		t.Fatalf("GetLatestRelease: %v", err)
	}
	if resp.Release != nil || resp.HasUpdate {
		t.Errorf("no release => %+v, want nil/false", resp)
	}
}

func TestGetReleaseByIDAndChangelog(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	id := seedRelease(t, db, "switch", "1.0.0", true, time.Now())

	rel, err := svc.GetReleaseByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetReleaseByID: %v", err)
	}
	if rel == nil || rel.Version != "1.0.0" {
		t.Fatalf("release = %+v, want 1.0.0", rel)
	}

	cl, err := svc.GetChangelog(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChangelog: %v", err)
	}
	if cl == "" {
		t.Error("expected non-empty changelog")
	}
}

func TestGenerateDownloadURL_NoStorageErrors(t *testing.T) {
	db := setupServiceTestDB(t)
	svc := newReleaseSvc(t, db)
	_, err := svc.GenerateDownloadURL(context.Background(), &entity.ReleaseArtifact{StoragePath: "x"})
	if err == nil {
		t.Fatal("expected error when object storage is not configured")
	}
}
