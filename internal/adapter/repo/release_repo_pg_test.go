package repo

// release_repo_pg_test.go — real-Postgres coverage for ReleaseRepository.
//
// ReleaseRepository is a straight GORM CRUD layer for the public product
// release / download catalogue. Every method was 0% covered. These tests run
// against the disposable PG (SetupTestDB self-isolates via a unique database)
// and assert concrete post-state rows — published/prerelease filtering, the
// atomic download-count increment, and the not-found → (nil, nil) contract —
// not just err==nil.

import (
	"context"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
)

// setupReleasePG stands up the disposable PG (skips without TEST_POSTGRES_DSN)
// and adds the release-catalogue tables, which SetupTestDB does not migrate.
func setupReleasePG(t *testing.T) {
	t.Helper()
	SetupTestDB(t) // t.Cleanup registered inside; skips when DSN unset
	if err := DB.AutoMigrate(&entity.Release{}, &entity.ReleaseArtifact{}, &entity.DownloadLog{}); err != nil {
		t.Fatalf("migrate release tables: %v", err)
	}
}

func mkRelease(t *testing.T, r *ReleaseRepository, productID, version string, published, prerelease bool) *entity.Release {
	t.Helper()
	now := time.Now()
	rel := &entity.Release{
		ProductId:    productID,
		Version:      version,
		Title:        productID + " " + version,
		ReleaseType:  "stable",
		IsPublished:  published,
		IsPrerelease: prerelease,
		ChangelogMd:  "## " + version + "\n- notes",
		PublishedAt:  &now,
	}
	if err := r.CreateRelease(context.Background(), rel); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
	if rel.Id == 0 {
		t.Fatal("CreateRelease did not populate Id")
	}
	return rel
}

func TestReleaseRepository_CRUDAndFilters(t *testing.T) {
	setupReleasePG(t)
	repo := NewReleaseRepository(DB)
	ctx := context.Background()

	// Seed: 2 published stable for productA, 1 prerelease productA, 1 unpublished,
	// 1 published stable for productB.
	relA1 := mkRelease(t, repo, "productA", "1.0.0", true, false)
	time.Sleep(2 * time.Millisecond)
	relA2 := mkRelease(t, repo, "productA", "1.1.0", true, false) // newer published_at
	mkRelease(t, repo, "productA", "2.0.0-rc1", true, true)       // prerelease
	mkRelease(t, repo, "productA", "0.9.0", false, false)         // unpublished
	relB1 := mkRelease(t, repo, "productB", "3.0.0", true, false)

	// Artifact on relA2 for download tests.
	art := &entity.ReleaseArtifact{
		ReleaseId:      relA2.Id,
		Platform:       "windows",
		Arch:           "x64",
		Filename:       "app.exe",
		FileSize:       1024,
		StoragePath:    "s3://bucket/app.exe",
		ChecksumSha256: "abc",
	}
	if err := repo.CreateArtifact(ctx, art); err != nil {
		t.Fatalf("CreateArtifact: %v", err)
	}

	t.Run("ListReleases excludes unpublished and prerelease by default", func(t *testing.T) {
		rels, total, err := repo.ListReleases(ctx, ListReleasesParams{ProductId: "productA"})
		if err != nil {
			t.Fatalf("ListReleases: %v", err)
		}
		// productA has 2 published-stable (unpublished + prerelease excluded).
		if total != 2 || len(rels) != 2 {
			t.Fatalf("want 2 published-stable productA releases, got total=%d len=%d", total, len(rels))
		}
		// Ordered published_at DESC → newest (1.1.0) first.
		if rels[0].Version != "1.1.0" {
			t.Errorf("want newest release first (1.1.0), got %q", rels[0].Version)
		}
		for _, r := range rels {
			if !r.IsPublished || r.IsPrerelease || r.ProductId != "productA" {
				t.Errorf("leaked non-matching release: %+v", r)
			}
		}
	})

	t.Run("ListReleases IncludePrerelease adds the rc", func(t *testing.T) {
		_, total, err := repo.ListReleases(ctx, ListReleasesParams{ProductId: "productA", IncludePrerelease: true})
		if err != nil {
			t.Fatalf("ListReleases: %v", err)
		}
		if total != 3 {
			t.Fatalf("want 3 with prerelease included, got %d", total)
		}
	})

	t.Run("ListReleases ReleaseType filter", func(t *testing.T) {
		_, total, err := repo.ListReleases(ctx, ListReleasesParams{ReleaseType: "beta"})
		if err != nil {
			t.Fatalf("ListReleases: %v", err)
		}
		if total != 0 {
			t.Fatalf("want 0 beta releases, got %d", total)
		}
	})

	t.Run("ListReleases pagination clamps", func(t *testing.T) {
		// Page/PageSize out of range are normalized; ask across all products.
		rels, total, err := repo.ListReleases(ctx, ListReleasesParams{Page: -5, PageSize: 999})
		if err != nil {
			t.Fatalf("ListReleases: %v", err)
		}
		// productA(2) + productB(1) published-stable = 3.
		if total != 3 || len(rels) != 3 {
			t.Fatalf("want 3 published-stable across products, got total=%d len=%d", total, len(rels))
		}
	})

	t.Run("GetLatestRelease returns newest published stable", func(t *testing.T) {
		latest, err := repo.GetLatestRelease(ctx, "productA")
		if err != nil {
			t.Fatalf("GetLatestRelease: %v", err)
		}
		if latest == nil || latest.Id != relA2.Id {
			t.Fatalf("want latest = relA2 (%d/1.1.0), got %+v", relA2.Id, latest)
		}
		_ = relA1
	})

	t.Run("GetLatestRelease missing product returns nil,nil", func(t *testing.T) {
		latest, err := repo.GetLatestRelease(ctx, "no-such-product")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if latest != nil {
			t.Fatalf("want nil for missing product, got %+v", latest)
		}
	})

	t.Run("GetReleaseByID published vs unpublished", func(t *testing.T) {
		got, err := repo.GetReleaseByID(ctx, relB1.Id)
		if err != nil {
			t.Fatalf("GetReleaseByID: %v", err)
		}
		if got == nil || got.Version != "3.0.0" {
			t.Fatalf("want relB1 3.0.0, got %+v", got)
		}
		// Unpublished id is filtered out → nil,nil.
		var unpub entity.Release
		if err := DB.Where("is_published = ?", false).First(&unpub).Error; err != nil {
			t.Fatalf("find unpublished: %v", err)
		}
		gotUn, err := repo.GetReleaseByID(ctx, unpub.Id)
		if err != nil {
			t.Fatalf("GetReleaseByID unpublished: %v", err)
		}
		if gotUn != nil {
			t.Fatalf("unpublished release must not be visible, got %+v", gotUn)
		}
	})

	t.Run("GetArtifactByID found and missing", func(t *testing.T) {
		gotArt, err := repo.GetArtifactByID(ctx, art.Id)
		if err != nil {
			t.Fatalf("GetArtifactByID: %v", err)
		}
		if gotArt == nil || gotArt.Filename != "app.exe" {
			t.Fatalf("want app.exe artifact, got %+v", gotArt)
		}
		missing, err := repo.GetArtifactByID(ctx, 999999)
		if err != nil {
			t.Fatalf("GetArtifactByID missing err: %v", err)
		}
		if missing != nil {
			t.Fatalf("want nil for missing artifact, got %+v", missing)
		}
	})

	t.Run("IncrementDownloadCount is persisted", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if err := repo.IncrementDownloadCount(ctx, art.Id); err != nil {
				t.Fatalf("IncrementDownloadCount: %v", err)
			}
		}
		var reloaded entity.ReleaseArtifact
		if err := DB.First(&reloaded, art.Id).Error; err != nil {
			t.Fatalf("reload artifact: %v", err)
		}
		if reloaded.DownloadCount != 3 {
			t.Fatalf("want download_count 3, got %d", reloaded.DownloadCount)
		}
	})

	t.Run("LogDownload persists a row", func(t *testing.T) {
		dl := &entity.DownloadLog{ArtifactId: art.Id, Status: "completed", CountryCode: "US", IpAddress: "127.0.0.1"}
		if err := repo.LogDownload(ctx, dl); err != nil {
			t.Fatalf("LogDownload: %v", err)
		}
		var cnt int64
		if err := DB.Model(&entity.DownloadLog{}).Where("artifact_id = ?", art.Id).Count(&cnt).Error; err != nil {
			t.Fatalf("count download logs: %v", err)
		}
		if cnt != 1 {
			t.Fatalf("want 1 download log, got %d", cnt)
		}
	})

	t.Run("GetChangelogByReleaseID returns markdown, empty for missing", func(t *testing.T) {
		md, err := repo.GetChangelogByReleaseID(ctx, relA2.Id)
		if err != nil {
			t.Fatalf("GetChangelogByReleaseID: %v", err)
		}
		if md == "" || md[:2] != "##" {
			t.Fatalf("want changelog markdown, got %q", md)
		}
		empty, err := repo.GetChangelogByReleaseID(ctx, 999999)
		if err != nil {
			t.Fatalf("GetChangelogByReleaseID missing: %v", err)
		}
		if empty != "" {
			t.Fatalf("want empty changelog for missing release, got %q", empty)
		}
	})
}
