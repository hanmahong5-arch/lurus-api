package nats

import "testing"

func TestTruncateRune(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short ascii", "hello", 80, "hello"},
		{"exact length", "12345", 5, "12345"},
		{"long ascii truncates", "abcdefghij", 5, "abcde"},
		{"empty", "", 80, ""},
		{"short cjk", "你好世界", 80, "你好世界"},
		{"long cjk truncates rune-safe", "你好世界一二三四五六七八九十", 5, "你好世界一"},
		{"mixed truncate at boundary", "abc你好defg", 5, "abc你好"},
		{"max=0 returns empty for non-empty", "anything", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateRune(tc.in, tc.max)
			if got != tc.want {
				t.Errorf("truncateRune(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestPublishImageGenerated_NoOpOnInvalidUser(t *testing.T) {
	// Should not panic; should silently no-op for userID <= 0.
	PublishImageGenerated(nil, 0, "gpt-4o", "test", "")
	PublishImageGenerated(nil, -1, "gpt-4o", "test", "")
}

func TestPublishUsageMilestone_NoOpOnInvalidUser(t *testing.T) {
	PublishUsageMilestone(nil, 0, 1000, "10k", "day")
	PublishUsageMilestone(nil, -5, 1000, "10k", "day")
}
