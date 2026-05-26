package types

import (
	"net/http"
	"strings"
)

// Token relay scopes. A token's Scopes field is a comma-separated subset of
// these constants. The TokenAuth middleware maps each incoming relay path
// to a required scope (via PathToRequiredScope) and rejects requests whose
// token does not carry that scope with 403 forbidden_scope.
//
// Empty scopes on a token means "no restriction" — preserves backward
// compatibility with every token issued before Phase E2 (migration 015).
const (
	TokenScopeChat      = "chat"      // /v1/chat/completions, /v1/completions, /v1/messages, /v1/responses, /v1/moderations, Gemini /v1beta/models/*
	TokenScopeEmbedding = "embedding" // /v1/embeddings, /v1/engines/:model/embeddings, /v1/rerank
	TokenScopeImage     = "image"     // /v1/images/*, /v1/edits
	TokenScopeAudio     = "audio"     // /v1/audio/transcriptions, /v1/audio/translations, /v1/audio/speech
	TokenScopeRealtime  = "realtime"  // /v1/realtime (WebSocket)
	TokenScopeMJ        = "mj"        // /mj/*, /:mode/mj/*
	TokenScopeTask      = "task"      // /suno/*, /v1/audio/music (async task relays)
	TokenScopeAdmin     = "admin"     // reserved for future admin-only token operations
)

// ValidTokenScopes enumerates every scope value accepted by ValidateTokenScopes.
// Order is stable so callers can iterate it deterministically for OpenAPI / UI
// dropdowns.
var ValidTokenScopes = []string{
	TokenScopeChat,
	TokenScopeEmbedding,
	TokenScopeImage,
	TokenScopeAudio,
	TokenScopeRealtime,
	TokenScopeMJ,
	TokenScopeTask,
	TokenScopeAdmin,
}

// IsValidTokenScope reports whether s is one of the defined scope constants.
// Empty string returns false — empty Scopes on a token has its own "no
// restriction" semantics handled by HasScope, separate from value validity.
func IsValidTokenScope(s string) bool {
	for _, v := range ValidTokenScopes {
		if s == v {
			return true
		}
	}
	return false
}

// PathToRequiredScope returns the scope required to access the given relay path.
// An empty return means no scope check applies (info endpoints like GET /v1/models,
// self-service billing endpoints, etc.).
//
// Method matters for /v1/models and /v1beta/models — the GET variants list/retrieve
// model metadata (no scope) while POST routes through them are Gemini chat relay.
func PathToRequiredScope(method, path string) string {
	// Suno + OpenAI-compatible music generation are async tasks — must precede
	// the generic /v1/audio/ branch so /v1/audio/music does not match audio.
	if strings.HasPrefix(path, "/v1/audio/music") || strings.HasPrefix(path, "/suno/") {
		return TokenScopeTask
	}

	// Midjourney has two route shapes: /mj/* and /:mode/mj/*. The substring
	// check covers both without parsing the mode segment.
	if strings.Contains(path, "/mj/") {
		return TokenScopeMJ
	}

	switch {
	case path == "/v1/realtime":
		return TokenScopeRealtime
	case strings.HasPrefix(path, "/v1/chat"),
		strings.HasPrefix(path, "/v1/completions"),
		strings.HasPrefix(path, "/v1/messages"),
		strings.HasPrefix(path, "/v1/responses"),
		strings.HasPrefix(path, "/v1/moderations"):
		return TokenScopeChat
	case strings.HasPrefix(path, "/v1beta/models/"),
		strings.HasPrefix(path, "/v1beta/openai/models/"):
		// GET on these paths lists models (info, no scope); POST hits the Gemini
		// chat relay through the *path catch-all. The router only registers POST
		// for the chat dispatch, but TokenAuth runs for both, so we filter here.
		if method == http.MethodGet {
			return ""
		}
		return TokenScopeChat
	case strings.HasPrefix(path, "/v1/models/"):
		// /v1/models/:model GET retrieves model info; POST /v1/models/*path goes
		// to the Gemini relay (see relay-router.go). Same filter as above.
		if method == http.MethodGet {
			return ""
		}
		return TokenScopeChat
	case strings.HasPrefix(path, "/v1/embeddings"),
		strings.Contains(path, "/engines/") && strings.Contains(path, "/embeddings"):
		return TokenScopeEmbedding
	case strings.HasPrefix(path, "/v1/rerank"):
		// Rerank is a similarity-task cousin of embeddings; lumped here so a
		// search-stack token doesn't need two unrelated scopes.
		return TokenScopeEmbedding
	case strings.HasPrefix(path, "/v1/images/"),
		path == "/v1/edits":
		return TokenScopeImage
	case strings.HasPrefix(path, "/v1/audio/"):
		return TokenScopeAudio
	}
	return ""
}
