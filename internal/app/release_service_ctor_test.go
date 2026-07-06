package app

// release_service_ctor_test.go — covers NewReleaseService's MinIO-configured
// construction path (the lowest-covered constructor arm) and ListReleases' repo
// error wrapping. minio.New is lazy (no dial), so pointing it at placeholder
// endpoints exercises both the internal and public presign-client init without
// a real object store.

import (
	"context"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/config"
)

// TestNewReleaseService_WithMinIOConfig drives the MinIO client + dedicated
// presign client initialization. Mutating the (pointer) global config's Storage
// block is the hermetic seam: config.Get() is memoized, but returns a live
// pointer we can override for the duration of the test.
func TestNewReleaseService_WithMinIOConfig(t *testing.T) {
	cfg := config.Get()
	prev := cfg.Storage
	cfg.Storage.MinIOEndpoint = "minio.internal:9000"
	cfg.Storage.MinIOPublicEndpoint = "https://downloads.example.com"
	cfg.Storage.MinIOAccessKey = "ak"
	cfg.Storage.MinIOSecretKey = "sk"
	cfg.Storage.MinIOBucket = "releases"
	cfg.Storage.MinIOSecure = false
	t.Cleanup(func() { cfg.Storage = prev })

	svc := NewReleaseService(nil)

	if !svc.IsStorageConfigured() {
		t.Fatal("IsStorageConfigured = false, want true with MinIOEndpoint set")
	}
	// A public endpoint was configured, so a dedicated presign client must exist
	// and be preferred over the internal client.
	if svc.presignClient() == svc.minioClient {
		t.Error("presignClient() returned the internal client; want the dedicated public presign client")
	}
	if svc.minioBucket != "releases" {
		t.Errorf("minioBucket = %q, want releases", svc.minioBucket)
	}
}

// TestListReleases_RepoErrorWrapped drives the repo-error arm: the release table
// is absent, so the underlying query fails and ListReleases wraps and returns the
// error (rather than a partial response).
func TestListReleases_RepoErrorWrapped(t *testing.T) {
	db := setupServiceTestDB(t) // does NOT migrate the release tables
	svc := NewReleaseService(repo.NewReleaseRepository(db))

	_, err := svc.ListReleases(context.Background(), repo.ListReleasesParams{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected an error listing releases when the table is missing")
	}
}
