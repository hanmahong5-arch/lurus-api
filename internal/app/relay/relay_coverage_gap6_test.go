package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
)

// TestRelayMidjourneySubmit_ActionModes drives the per-relay-mode action
// assignment branches of RelayMidjourneySubmit (describe / blend / shorten /
// edits / upload). Each mode maps to a distinct MJ action and then dispatches to
// the fake upstream; the submit must succeed (nil response) and persist the task
// under the upstream-issued MjId. This exercises the mode→action switch arms
// that the imagine/change happy paths do not.
func TestRelayMidjourneySubmit_ActionModes(t *testing.T) {
	modes := []struct {
		name string
		mode int
	}{
		{"describe", relayconstant.RelayModeMidjourneyDescribe},
		{"blend", relayconstant.RelayModeMidjourneyBlend},
		{"shorten", relayconstant.RelayModeMidjourneyShorten},
		{"edits", relayconstant.RelayModeMidjourneyEdits},
		{"upload", relayconstant.RelayModeMidjourneyUpload},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			cleanup := setupRelayDB(t)
			defer cleanup()
			app.InitHttpClient()

			u := &repo.User{Username: "mj-mode-" + m.name, Quota: 100_000_000}
			if err := repo.DB.Create(u).Error; err != nil {
				t.Fatalf("seed user: %v", err)
			}

			mjID := "mj-mode-" + m.name
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":1,"result":"` + mjID + `","description":"ok"}`))
			}))
			defer srv.Close()

			body := []byte(`{"prompt":"p","base64Array":["data:image/png;base64,AAAA"],"imageUrl":"http://x/y.png","base64":"data:image/png;base64,AAAA"}`)
			c, _ := mjPostContext(t, "/mj/submit/"+m.name, body)
			c.Set("base_url", srv.URL)
			c.Set("token_name", "tkn")

			info := &relaycommon.RelayInfo{UserId: u.Id, IsPlayground: true}
			info.RelayMode = m.mode

			resp := RelayMidjourneySubmit(c, info)
			if resp != nil {
				t.Fatalf("mode %s: expected nil (success), got %+v", m.name, resp)
			}
			if saved := repo.GetByOnlyMJId(mjID); saved == nil {
				t.Fatalf("mode %s: expected persisted task %s", m.name, mjID)
			}
		})
	}
}
