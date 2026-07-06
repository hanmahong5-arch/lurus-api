package common

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/gin-gonic/gin"
)

// ---------- gin.go response + body helpers ----------

func TestApiResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ApiError(c, errors.New("boom"))
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["success"] != false || got["message"] != "boom" {
		t.Errorf("ApiError body = %v", got)
	}

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	ApiErrorMsg(c2, "nope")
	var got2 map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &got2)
	if got2["message"] != "nope" {
		t.Errorf("ApiErrorMsg body = %v", got2)
	}

	rec3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(rec3)
	ApiSuccess(c3, map[string]int{"n": 1})
	var got3 map[string]any
	_ = json.Unmarshal(rec3.Body.Bytes(), &got3)
	if got3["success"] != true {
		t.Errorf("ApiSuccess body = %v", got3)
	}
}

func TestIsRequestBodyTooLargeError(t *testing.T) {
	if IsRequestBodyTooLargeError(nil) {
		t.Error("nil is not too-large")
	}
	if !IsRequestBodyTooLargeError(ErrRequestBodyTooLarge) {
		t.Error("sentinel should be recognized")
	}
	if IsRequestBodyTooLargeError(errors.New("other")) {
		t.Error("unrelated error should not match")
	}
}

func TestGetRequestBodyAndUnmarshalReusable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origMax := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 10
	t.Cleanup(func() { constant.MaxRequestBodyMB = origMax })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"neo"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	body, err := GetRequestBody(c)
	if err != nil || string(body) != `{"name":"neo"}` {
		t.Fatalf("GetRequestBody = %q err=%v", body, err)
	}
	// Cached on second call (body already consumed).
	body2, err := GetRequestBody(c)
	if err != nil || string(body2) != `{"name":"neo"}` {
		t.Fatalf("cached GetRequestBody = %q err=%v", body2, err)
	}

	var dst struct {
		Name string `json:"name"`
	}
	if err := UnmarshalBodyReusable(c, &dst); err != nil || dst.Name != "neo" {
		t.Fatalf("UnmarshalBodyReusable = %+v err=%v", dst, err)
	}
	// Body must be re-readable after reusable unmarshal.
	again, _ := GetRequestBody(c)
	if string(again) != `{"name":"neo"}` {
		t.Errorf("body not reusable: %q", again)
	}
}

func TestGetContextKeyStringMapAndTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("m", map[string]any{"a": 1})
	if GetContextKeyStringMap(c, constant.ContextKey("m"))["a"] != 1 {
		t.Error("string map getter mismatch")
	}
	// Absent time key returns zero value without panic.
	if !GetContextKeyTime(c, constant.ContextKey("no-time")).IsZero() {
		t.Error("absent time key should be zero")
	}
}

// ---------- gopool.RelayCtxGo ----------

func TestRelayCtxGo(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ran := false
	RelayCtxGo(context.Background(), func() {
		ran = true
		wg.Done()
	})
	wg.Wait()
	if !ran {
		t.Error("RelayCtxGo did not execute the function")
	}
}

// ---------- leader.go ----------

func TestLeaderState(t *testing.T) {
	orig := IsLeader()
	t.Cleanup(func() { SetLeader(orig) })

	SetLeader(true)
	if !IsLeader() {
		t.Error("SetLeader(true) not reflected")
	}
	SetLeader(false)
	if IsLeader() {
		t.Error("SetLeader(false) not reflected")
	}
}

func TestNodeHolderID(t *testing.T) {
	id := NodeHolderID()
	if id == "" {
		t.Fatal("node holder id empty")
	}
	// Stable within a process.
	if NodeHolderID() != id {
		t.Error("node holder id should be stable")
	}
	if !strings.Contains(id, "-") {
		t.Errorf("expected host-suffix format, got %q", id)
	}
}
