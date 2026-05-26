package repo

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

// HasScope and GetScopes guard the central authorization decision made by
// TokenAuth (middleware/auth.go). Empty Scopes must allow every relay path
// (backward compat with every token issued before migration 015) and a
// non-empty allowlist must reject any scope not in it.
func TestToken_HasScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes string
		probe  string
		want   bool
	}{
		// Empty allowlist → unrestricted. This is the contract every existing
		// token in stage/prod relies on; if it ever flips, every relay key
		// issued before Phase E2 stops working overnight.
		{"empty scopes allows chat", "", types.TokenScopeChat, true},
		{"empty scopes allows image", "", types.TokenScopeImage, true},
		{"empty scopes allows admin", "", types.TokenScopeAdmin, true},

		// Single-scope allowlists.
		{"chat token allows chat", types.TokenScopeChat, types.TokenScopeChat, true},
		{"chat token rejects image", types.TokenScopeChat, types.TokenScopeImage, false},
		{"chat token rejects audio", types.TokenScopeChat, types.TokenScopeAudio, false},

		// Multi-scope allowlists.
		{"chat+embedding allows chat", "chat,embedding", types.TokenScopeChat, true},
		{"chat+embedding allows embedding", "chat,embedding", types.TokenScopeEmbedding, true},
		{"chat+embedding rejects image", "chat,embedding", types.TokenScopeImage, false},

		// Whitespace tolerance — comma-and-space is a common UI input shape.
		{"whitespace separator works", "chat, embedding ,  image", types.TokenScopeImage, true},
		{"whitespace separator still rejects", "chat, embedding", types.TokenScopeAudio, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := &Token{Scopes: tc.scopes}
			if got := tok.HasScope(tc.probe); got != tc.want {
				t.Errorf("HasScope(%q) on scopes=%q = %v, want %v", tc.probe, tc.scopes, got, tc.want)
			}
		})
	}
}

func TestToken_GetScopes(t *testing.T) {
	cases := []struct {
		scopes string
		want   []string
	}{
		{"", nil},
		{"chat", []string{"chat"}},
		{"chat,embedding", []string{"chat", "embedding"}},
		{"  chat , embedding ", []string{"chat", "embedding"}},
		{",,chat,,", []string{"chat"}}, // empty entries are dropped
	}
	for _, tc := range cases {
		t.Run(tc.scopes, func(t *testing.T) {
			tok := &Token{Scopes: tc.scopes}
			got := tok.GetScopes()
			if len(got) != len(tc.want) {
				t.Fatalf("GetScopes() length = %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("GetScopes()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
