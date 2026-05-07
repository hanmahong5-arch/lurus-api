package app

import "testing"

func TestParseCostSpikeMember(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1714800000000:1234", 1234},
		{"1714800000000:0", 0},
		{"1714800000000:9999999", 9999999},
		{"malformed", 0},
		{"", 0},
		{":1234", 1234},
		{"1714800000000:notanumber", 0},
		{"1714800000000:1234:extra", 0}, // SplitN(":", 2) leaves "1234:extra" in field 2 → ParseInt fails
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := parseCostSpikeMember(tc.in); got != tc.want {
				t.Errorf("parseCostSpikeMember(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
