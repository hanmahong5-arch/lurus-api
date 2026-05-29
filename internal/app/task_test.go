package app

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
)

// CoverTaskActionToModelName is a pure string transform: "<platform>_<action>",
// both lowercased.
func TestCoverTaskActionToModelName(t *testing.T) {
	tests := []struct {
		name     string
		platform constant.TaskPlatform
		action   string
		want     string
	}{
		{"suno_lowercases_action", constant.TaskPlatformSuno, "MUSIC", "suno_music"},
		{"midjourney_short_platform", constant.TaskPlatform(constant.TaskPlatformMidjourney), "IMAGINE", "mj_imagine"},
		{"music_mixed_case", constant.TaskPlatformMusic, "Generate", "music_generate"},
		{"already_lower", constant.TaskPlatformSuno, "lyrics", "suno_lyrics"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoverTaskActionToModelName(tc.platform, tc.action); got != tc.want {
				t.Errorf("CoverTaskActionToModelName(%q, %q) = %q, want %q", tc.platform, tc.action, got, tc.want)
			}
		})
	}
}
