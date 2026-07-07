package common

import (
	"embed"
	"testing"
)

//go:embed testdata/embedroot
var testEmbedFS embed.FS

func TestEmbedFolder(t *testing.T) {
	sfs := EmbedFolder(testEmbedFS, "testdata/embedroot")

	// Exists uses Open under the hood; the embedded file must be found.
	if !sfs.Exists("/", "/file.txt") {
		t.Error("embedded file.txt should exist")
	}
	if sfs.Exists("/", "/missing.txt") {
		t.Error("missing file should not exist")
	}

	// Open on a real file returns a handle; Open("/") is redirected to NotExist
	// so the SPA index falls through to the NoRoute handler.
	f, err := sfs.Open("/file.txt")
	if err != nil {
		t.Fatalf("open embedded file: %v", err)
	}
	_ = f.Close()
	if _, err := sfs.Open("/"); err == nil {
		t.Error(`Open("/") must return an error so index goes to NoRoute`)
	}
}

func TestInitZitaClient_DisabledWhenUnset(t *testing.T) {
	orig := ZitaClient
	t.Cleanup(func() { ZitaClient = orig })
	ZitaClient = nil

	t.Setenv("IDENTITY_PUBLIC_URL", "")
	t.Setenv("IDENTITY_SESSION_SECRET", "")
	InitZitaClient()
	if ZitaClient != nil {
		t.Error("zita client must stay nil when identity env is unset")
	}
}
