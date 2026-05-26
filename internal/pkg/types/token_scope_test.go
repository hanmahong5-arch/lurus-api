package types

import (
	"net/http"
	"testing"
)

func TestIsValidTokenScope(t *testing.T) {
	cases := []struct {
		scope string
		want  bool
	}{
		{TokenScopeChat, true},
		{TokenScopeEmbedding, true},
		{TokenScopeImage, true},
		{TokenScopeAudio, true},
		{TokenScopeRealtime, true},
		{TokenScopeMJ, true},
		{TokenScopeTask, true},
		{TokenScopeAdmin, true},
		{"", false},
		{"writeall", false},
		{"chat,embedding", false}, // comma-joined is not a scope name
		{"Chat", false},           // case-sensitive
	}
	for _, tc := range cases {
		t.Run(tc.scope, func(t *testing.T) {
			if got := IsValidTokenScope(tc.scope); got != tc.want {
				t.Errorf("IsValidTokenScope(%q) = %v, want %v", tc.scope, got, tc.want)
			}
		})
	}
}

func TestPathToRequiredScope(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		// chat group — every text-generation surface lands here so a chat
		// token works for OpenAI / Anthropic / Gemini equally
		{"openai chat completions", http.MethodPost, "/v1/chat/completions", TokenScopeChat},
		{"openai legacy completions", http.MethodPost, "/v1/completions", TokenScopeChat},
		{"claude messages", http.MethodPost, "/v1/messages", TokenScopeChat},
		{"openai responses", http.MethodPost, "/v1/responses", TokenScopeChat},
		{"moderations", http.MethodPost, "/v1/moderations", TokenScopeChat},
		{"gemini relay v1beta", http.MethodPost, "/v1beta/models/gemini-pro:generateContent", TokenScopeChat},
		{"gemini openai-compat", http.MethodPost, "/v1beta/openai/models/gemini-pro:generateContent", TokenScopeChat},
		{"openai models relay POST", http.MethodPost, "/v1/models/gemini-pro:generateContent", TokenScopeChat},

		// embedding group (rerank lumped in deliberately — same scope keeps
		// search stacks simple)
		{"openai embeddings", http.MethodPost, "/v1/embeddings", TokenScopeEmbedding},
		{"engines embeddings legacy", http.MethodPost, "/v1/engines/text-embedding-ada-002/embeddings", TokenScopeEmbedding},
		{"rerank", http.MethodPost, "/v1/rerank", TokenScopeEmbedding},

		// image group
		{"images generations", http.MethodPost, "/v1/images/generations", TokenScopeImage},
		{"images edits", http.MethodPost, "/v1/images/edits", TokenScopeImage},
		{"edits root", http.MethodPost, "/v1/edits", TokenScopeImage},

		// audio (TTS/ASR — not music)
		{"audio transcriptions", http.MethodPost, "/v1/audio/transcriptions", TokenScopeAudio},
		{"audio translations", http.MethodPost, "/v1/audio/translations", TokenScopeAudio},
		{"audio speech", http.MethodPost, "/v1/audio/speech", TokenScopeAudio},

		// realtime
		{"realtime ws", http.MethodGet, "/v1/realtime", TokenScopeRealtime},

		// midjourney (both shapes)
		{"mj submit", http.MethodPost, "/mj/submit/imagine", TokenScopeMJ},
		{"mj mode submit", http.MethodPost, "/fast/mj/submit/imagine", TokenScopeMJ},

		// task (suno + audio music — both async, distinct from audio TTS)
		{"suno submit", http.MethodPost, "/suno/submit/music", TokenScopeTask},
		{"audio music (precedes audio)", http.MethodPost, "/v1/audio/music", TokenScopeTask},
		{"audio music status", http.MethodGet, "/v1/audio/music/task-id-123", TokenScopeTask},

		// info / no-scope (GET on model listing)
		{"models list GET", http.MethodGet, "/v1/models", ""},
		{"models retrieve GET", http.MethodGet, "/v1/models/gpt-4", ""},
		{"gemini models list GET", http.MethodGet, "/v1beta/models", ""},
		{"gemini model GET", http.MethodGet, "/v1beta/models/gemini-pro", ""},
		{"billing balance", http.MethodGet, "/v1/billing/balance", ""},
		{"billing usage", http.MethodGet, "/v1/billing/usage", ""},
		{"unknown path", http.MethodPost, "/v1/something/new", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PathToRequiredScope(tc.method, tc.path); got != tc.want {
				t.Errorf("PathToRequiredScope(%s, %q) = %q, want %q", tc.method, tc.path, got, tc.want)
			}
		})
	}
}
