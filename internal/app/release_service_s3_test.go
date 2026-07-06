package app

// release_service_s3_test.go — drives the MinIO-backed manifest paths
// (discoverBinaryTools, readChecksums, presignClient, GenerateDownloadURL
// success) against an in-process fake S3 endpoint. minio-go signs presigned
// URLs locally when a Region is set, and ListObjectsV2/GetObject are plain
// path-style HTTP, so an httptest server is sufficient — no real MinIO needed.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// fakeS3 is a minimal path-style S3 server: it answers ListObjectsV2 with a
// fixed object set and serves object bodies for GET requests. Unknown objects
// return 404. When listErr is true, list requests return 500 so the caller's
// error branch runs.
type fakeS3 struct {
	bucket  string
	objects map[string]string // key -> body
	sizes   map[string]int64
	listErr bool
}

func (f *fakeS3) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ListObjectsV2: GET /{bucket}?list-type=2&prefix=...
		if r.Method == http.MethodGet && r.URL.Query().Get("list-type") != "" {
			if f.listErr {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			prefix := r.URL.Query().Get("prefix")
			var b strings.Builder
			b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
			b.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
			b.WriteString("<Name>" + f.bucket + "</Name>")
			b.WriteString("<Prefix>" + prefix + "</Prefix>")
			b.WriteString("<KeyCount>0</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated>")
			for key, body := range f.objects {
				if !strings.HasPrefix(key, prefix) {
					continue
				}
				size := f.sizes[key]
				if size == 0 {
					size = int64(len(body))
				}
				b.WriteString("<Contents>")
				b.WriteString("<Key>" + key + "</Key>")
				b.WriteString("<LastModified>2024-01-01T00:00:00.000Z</LastModified>")
				b.WriteString(`<ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>`)
				b.WriteString(fmt.Sprintf("<Size>%d</Size>", size))
				b.WriteString("<StorageClass>STANDARD</StorageClass>")
				b.WriteString("</Contents>")
			}
			b.WriteString("</ListBucketResult>")
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(b.String()))
			return
		}

		// GetObject: GET /{bucket}/{key}
		key := strings.TrimPrefix(r.URL.Path, "/"+f.bucket+"/")
		body, ok := f.objects[key]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// minio-go requires a valid Last-Modified + Content-Length on object reads.
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
		w.Header().Set("Accept-Ranges", "bytes")
		_, _ = w.Write([]byte(body))
	}
}

func newFakeS3Service(t *testing.T, f *fakeS3) *ReleaseService {
	t.Helper()
	ts := httptest.NewServer(f.handler())
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test url: %v", err)
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4("testkey", "testsecret", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	return &ReleaseService{minioClient: client, minioBucket: f.bucket}
}

func TestDiscoverBinaryTools_BuildsManifestWithChecksums(t *testing.T) {
	f := &fakeS3{
		bucket: "artifacts",
		objects: map[string]string{
			// latest version of switch: two platforms + checksums
			"tools/switch/1.0.0/switch-linux-amd64.tar.gz": "linux-body",
			"tools/switch/1.0.0/switch-windows-amd64.exe":  "win-body",
			"tools/switch/1.0.0/checksums.sha256":          "aaa111  switch-linux-amd64.tar.gz\nbbb222 *switch-windows-amd64.exe\n",
			"tools/switch/0.9.0/switch-linux-amd64.tar.gz": "old-body", // older, must be ignored
			// creator tool without a checksums file → assets carry no SHA256
			"tools/creator/2.0.0/creator-darwin-arm64.tar.gz": "creator-body",
		},
		sizes: map[string]int64{
			"tools/switch/1.0.0/switch-linux-amd64.tar.gz": 100,
		},
	}
	svc := newFakeS3Service(t, f)

	tools, err := svc.discoverBinaryTools(context.Background())
	if err != nil {
		t.Fatalf("discoverBinaryTools: %v", err)
	}

	sw, ok := tools["switch"]
	if !ok {
		t.Fatal("manifest missing switch tool")
	}
	if sw.Type != "binary" {
		t.Errorf("switch type = %q, want binary", sw.Type)
	}
	if sw.LatestVersion != "1.0.0" {
		t.Errorf("switch latest = %q, want 1.0.0 (0.9.0 must lose)", sw.LatestVersion)
	}
	linux, ok := sw.Platforms["linux/amd64"]
	if !ok {
		t.Fatalf("switch missing linux/amd64 platform, got %+v", sw.Platforms)
	}
	if linux.SHA256 != "aaa111" {
		t.Errorf("linux/amd64 sha256 = %q, want aaa111 (from checksums.sha256)", linux.SHA256)
	}
	if linux.Size != 100 {
		t.Errorf("linux/amd64 size = %d, want 100", linux.Size)
	}
	if !strings.Contains(linux.URL, "X-Amz-Signature") {
		t.Errorf("linux URL not presigned: %q", linux.URL)
	}
	win, ok := sw.Platforms["windows/amd64"]
	if !ok {
		t.Fatalf("switch missing windows/amd64 platform")
	}
	if win.SHA256 != "bbb222" {
		t.Errorf("windows sha256 = %q, want bbb222 (star-prefixed name stripped)", win.SHA256)
	}
	// checksums.sha256 itself must not become a platform entry.
	if len(sw.Platforms) != 2 {
		t.Errorf("switch platform count = %d, want 2 (checksum file excluded)", len(sw.Platforms))
	}

	// creator has no checksum file → SHA256 empty but still listed.
	cr, ok := tools["creator"]
	if !ok {
		t.Fatal("manifest missing creator tool")
	}
	da, ok := cr.Platforms["darwin/arm64"]
	if !ok {
		t.Fatalf("creator missing darwin/arm64, got %+v", cr.Platforms)
	}
	if da.SHA256 != "" {
		t.Errorf("creator sha256 = %q, want empty (no checksums file)", da.SHA256)
	}
}

func TestBuildToolManifest_MergesNpmAndBinary(t *testing.T) {
	f := &fakeS3{
		bucket: "artifacts",
		objects: map[string]string{
			"tools/switch/1.0.0/switch-linux-amd64.tar.gz": "body",
		},
	}
	svc := newFakeS3Service(t, f)

	resp, err := svc.BuildToolManifest(context.Background())
	if err != nil {
		t.Fatalf("BuildToolManifest: %v", err)
	}
	// 4 npm tools + 1 discovered binary tool.
	if len(resp.Tools) != 5 {
		t.Errorf("manifest tool count = %d, want 5 (4 npm + switch)", len(resp.Tools))
	}
	if _, ok := resp.Tools["switch"]; !ok {
		t.Error("manifest missing discovered switch binary")
	}
	if _, ok := resp.Tools["claude"]; !ok {
		t.Error("manifest missing static claude npm tool")
	}
}

func TestBuildToolManifest_ListErrorFallsBackToNpmOnly(t *testing.T) {
	f := &fakeS3{bucket: "artifacts", listErr: true}
	svc := newFakeS3Service(t, f)

	// discoverBinaryTools surfaces the list error; BuildToolManifest logs it and
	// returns the npm-only manifest rather than failing.
	resp, err := svc.BuildToolManifest(context.Background())
	if err != nil {
		t.Fatalf("BuildToolManifest should not fail on storage list error: %v", err)
	}
	if len(resp.Tools) != 4 {
		t.Errorf("manifest tool count = %d, want 4 (npm-only after list error)", len(resp.Tools))
	}

	// discoverBinaryTools directly must return the error.
	if _, derr := svc.discoverBinaryTools(context.Background()); derr == nil {
		t.Error("discoverBinaryTools should return the list error")
	}
}

func TestReadChecksums_MissingFileReturnsEmpty(t *testing.T) {
	f := &fakeS3{bucket: "artifacts", objects: map[string]string{}}
	svc := newFakeS3Service(t, f)
	// No checksums object exists → GetObject read 404 → empty result (no panic).
	got := svc.readChecksums(context.Background(), "switch", "1.0.0")
	if len(got) != 0 {
		t.Errorf("readChecksums for missing file = %v, want empty", got)
	}
}

func TestGenerateDownloadURL_PresignsWhenConfigured(t *testing.T) {
	f := &fakeS3{bucket: "artifacts", objects: map[string]string{}}
	svc := newFakeS3Service(t, f)

	url, err := svc.GenerateDownloadURL(context.Background(), &entity.ReleaseArtifact{StoragePath: "tools/switch/1.0.0/switch-linux-amd64.tar.gz"})
	if err != nil {
		t.Fatalf("GenerateDownloadURL: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("download URL not presigned: %q", url)
	}
	if !strings.Contains(url, "switch-linux-amd64.tar.gz") {
		t.Errorf("download URL missing object path: %q", url)
	}
}

func TestPresignClient_PrefersDedicatedPresignClient(t *testing.T) {
	f := &fakeS3{bucket: "artifacts", objects: map[string]string{}}
	svc := newFakeS3Service(t, f)
	// Without a dedicated presign client, presignClient() falls back to minioClient.
	if svc.presignClient() != svc.minioClient {
		t.Error("presignClient should fall back to minioClient when presign client is nil")
	}
	// With a dedicated presign client set, it is preferred.
	pc, _ := minio.New("example.com", &minio.Options{
		Creds:  credentials.NewStaticV4("k", "s", ""),
		Region: "us-east-1",
	})
	svc.minioPresignClient = pc
	if svc.presignClient() != pc {
		t.Error("presignClient should prefer the dedicated presign client")
	}
}
