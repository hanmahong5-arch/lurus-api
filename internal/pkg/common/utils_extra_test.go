package common

import (
	"os"
	"strings"
	"testing"
)

func TestBytes2Size(t *testing.T) {
	cases := map[int64]string{
		512:                    "512 B",
		2048:                   "2 KB",
		5 * 1024 * 1024:        "5 MB",
		3 * 1024 * 1024 * 1024: "3.00 GB",
	}
	for in, want := range cases {
		if got := Bytes2Size(in); got != want {
			t.Errorf("Bytes2Size(%d)=%q want %q", in, got, want)
		}
	}
}

func TestSeconds2Time(t *testing.T) {
	if got := Seconds2Time(45); got != "45 秒" {
		t.Errorf("45s => %q", got)
	}
	// 1 day + 1 hour + 1 minute + 1 second.
	got := Seconds2Time(86400 + 3600 + 60 + 1)
	for _, want := range []string{"1 天", "1 小时", "1 分钟", "1 秒"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestInterface2String(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{"str", "str"},
		{42, "42"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := Interface2String(c.in); got != c.want {
			t.Errorf("Interface2String(%v)=%q want %q", c.in, got, c.want)
		}
	}
	// float and fallback branches produce non-empty output.
	if Interface2String(1.5) == "" {
		t.Error("float should stringify")
	}
	if Interface2String([]int{1}) == "" {
		t.Error("slice fallback should stringify")
	}
}

func TestIntMaxAndMax(t *testing.T) {
	if IntMax(3, 5) != 5 || IntMax(9, 2) != 9 {
		t.Error("IntMax wrong")
	}
	if Max(3, 5) != 5 || Max(9, 2) != 9 {
		t.Error("Max wrong")
	}
}

func TestGetUUID(t *testing.T) {
	u := GetUUID()
	if len(u) != 32 {
		t.Errorf("UUID length = %d, want 32", len(u))
	}
	if strings.Contains(u, "-") {
		t.Error("UUID should have dashes stripped")
	}
	if GetUUID() == u {
		t.Error("two UUIDs should differ")
	}
}

func TestGenerateRandomCharsKey(t *testing.T) {
	k, err := GenerateRandomCharsKey(48)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(k) != 48 {
		t.Errorf("length = %d, want 48", len(k))
	}
	for _, ch := range k {
		if !strings.ContainsRune(keyChars, ch) {
			t.Errorf("char %q not in keyChars", ch)
		}
	}
}

func TestGenerateRandomKeyAndGenerateKey(t *testing.T) {
	k, err := GenerateRandomKey(48)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if k == "" {
		t.Error("random key empty")
	}
	gk, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	if len(gk) != 48 {
		t.Errorf("GenerateKey length = %d, want 48", len(gk))
	}
}

func TestGetRandomInt(t *testing.T) {
	for i := 0; i < 50; i++ {
		if v := GetRandomInt(10); v < 0 || v >= 10 {
			t.Fatalf("GetRandomInt out of range: %d", v)
		}
	}
}

func TestGetTimestampAndTimeString(t *testing.T) {
	if GetTimestamp() <= 0 {
		t.Error("timestamp should be positive")
	}
	if len(GetTimeString()) < 14 {
		t.Errorf("time string too short: %q", GetTimeString())
	}
}

func TestMessageWithRequestId(t *testing.T) {
	got := MessageWithRequestId("boom", "req-42")
	if !strings.Contains(got, "boom") || !strings.Contains(got, "req-42") {
		t.Errorf("unexpected: %q", got)
	}
}

func TestGetPointer(t *testing.T) {
	p := GetPointer(7)
	if p == nil || *p != 7 {
		t.Error("GetPointer wrong")
	}
}

func TestAny2Type(t *testing.T) {
	type target struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	got, err := Any2Type[target](map[string]interface{}{"a": 1, "b": "x"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.A != 1 || got.B != "x" {
		t.Errorf("unexpected: %+v", got)
	}
	// Type mismatch on unmarshal must error.
	if _, err := Any2Type[target]("not-an-object"); err == nil {
		t.Error("expected error for incompatible type")
	}
}

func TestUnescapeHTML(t *testing.T) {
	if UnescapeHTML("<b>") == nil {
		t.Error("UnescapeHTML returned nil")
	}
}

func TestBuildURL(t *testing.T) {
	cases := []struct {
		base, endpoint, want string
	}{
		{"https://api.example.com", "/v1/chat", "https://api.example.com/v1/chat"},
		{"https://api.example.com/", "/v1/chat", "https://api.example.com/v1/chat"},
		{"https://api.example.com/base/", "v1", "https://api.example.com/base/v1"},
		{"https://api.example.com", "", "https://api.example.com/"},
	}
	for _, c := range cases {
		if got := BuildURL(c.base, c.endpoint); got != c.want {
			t.Errorf("BuildURL(%q,%q)=%q want %q", c.base, c.endpoint, got, c.want)
		}
	}
}

func TestSaveTmpFile(t *testing.T) {
	name, err := SaveTmpFile("unit-tmp-*.txt", strings.NewReader("content-here"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if name == "" {
		t.Error("empty tmp file name")
	}
	t.Cleanup(func() { _ = os.Remove(name) })
}
