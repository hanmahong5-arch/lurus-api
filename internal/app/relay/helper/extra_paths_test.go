package helper

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestGetAndValidOpenAIImageRequest_MultipartEdit(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("model", "gpt-image-1")
	_ = mw.WriteField("prompt", "make it blue")
	_ = mw.WriteField("n", "2")
	_ = mw.WriteField("size", "1024x1024")
	_ = mw.WriteField("image", "base64data")
	_ = mw.WriteField("watermark", "true")
	_ = mw.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/v1/images/edits", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req

	imgReq, err := GetAndValidOpenAIImageRequest(c, 6 /*RelayModeImagesEdits*/)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if imgReq.Model != "gpt-image-1" {
		t.Errorf("model = %q", imgReq.Model)
	}
	if imgReq.Prompt != "make it blue" {
		t.Errorf("prompt = %q", imgReq.Prompt)
	}
	if imgReq.N != 2 {
		t.Errorf("N = %d, want 2", imgReq.N)
	}
	if imgReq.Quality != "standard" {
		t.Errorf("gpt-image-1 quality default = %q, want standard", imgReq.Quality)
	}
	if imgReq.Watermark == nil || *imgReq.Watermark != true {
		t.Errorf("watermark = %v, want true", imgReq.Watermark)
	}
}

func TestGetAndValidOpenAIImageRequest_EditFallthroughJSON(t *testing.T) {
	// Non-multipart edit request falls through to the JSON default branch.
	c, _ := newJSONCtx("POST", "/v1/images/edits", `{"model":"dall-e-2","size":"512x512"}`)
	imgReq, err := GetAndValidOpenAIImageRequest(c, 6 /*RelayModeImagesEdits*/)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if imgReq.Size != "512x512" {
		t.Errorf("size = %q", imgReq.Size)
	}
	if imgReq.N != 1 {
		t.Errorf("N default = %d", imgReq.N)
	}
}

// wsEchoServer spins up a websocket endpoint that upgrades and drains messages so
// the client side of Wss* helpers has a live peer to write to.
func wsDial(t *testing.T) (*websocket.Conn, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}
	return conn, func() { _ = conn.Close(); srv.Close() }
}

func TestWssHelpers(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	t.Run("nil connection errors", func(t *testing.T) {
		if err := WssString(c, nil, "hi"); err == nil {
			t.Error("WssString nil conn should error")
		}
		if err := WssObject(c, nil, map[string]int{"a": 1}); err == nil {
			t.Error("WssObject nil conn should error")
		}
		// WssError with nil conn just returns (no panic).
		WssError(c, nil, types.OpenAIError{Message: "x"})
	})

	t.Run("live connection writes succeed", func(t *testing.T) {
		conn, closeFn := wsDial(t)
		defer closeFn()

		if err := WssString(c, conn, "hello"); err != nil {
			t.Errorf("WssString: %v", err)
		}
		if err := WssObject(c, conn, map[string]int{"a": 1}); err != nil {
			t.Errorf("WssObject: %v", err)
		}
		// exercises the marshal + write path with a request id in context
		c.Set("X-Oneapi-Request-Id", "rid")
		WssError(c, conn, types.OpenAIError{Message: "boom", Type: "test"})
	})
}
