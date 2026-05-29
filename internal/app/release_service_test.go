package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{
			name:     "equal_versions",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "v1_less_than_v2",
			v1:       "1.0.0",
			v2:       "1.1.0",
			expected: -1,
		},
		{
			name:     "v1_greater_than_v2",
			v1:       "2.0.0",
			v2:       "1.9.0",
			expected: 1,
		},
		{
			name:     "patch_version_comparison",
			v1:       "1.0.1",
			v2:       "1.0.0",
			expected: 1,
		},
		{
			name:     "major_version_difference",
			v1:       "2.0.0",
			v2:       "1.99.99",
			expected: 1,
		},
		{
			name:     "with_v_prefix",
			v1:       "v1.0.0",
			v2:       "v1.1.0",
			expected: -1,
		},
		{
			name:     "mixed_prefix",
			v1:       "1.0.0",
			v2:       "v1.0.0",
			expected: 0,
		},
		{
			name:     "critical_bug_10_vs_9",
			v1:       "1.10.0",
			v2:       "1.9.0",
			expected: 1, // FIXED: Was -1 with string comparison
		},
		{
			name:     "double_digit_major",
			v1:       "10.0.0",
			v2:       "9.0.0",
			expected: 1, // FIXED: Was -1 with string comparison
		},
		{
			name:     "prerelease_alpha",
			v1:       "1.0.0-alpha",
			v2:       "1.0.0",
			expected: -1,
		},
		{
			name:     "prerelease_beta_vs_alpha",
			v1:       "1.0.0-beta",
			v2:       "1.0.0-alpha",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			assert.Equal(t, tt.expected, result,
				"compareVersions(%s, %s) = %d, expected %d",
				tt.v1, tt.v2, result, tt.expected)
		})
	}
}

func TestCompareVersions_NonSemver(t *testing.T) {
	// Test fallback for non-semver formats
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{
			name:     "equal_non_semver",
			v1:       "latest",
			v2:       "latest",
			expected: 0,
		},
		{
			name:     "different_non_semver",
			v1:       "beta",
			v2:       "alpha",
			expected: 1, // String comparison fallback
		},
		{
			name:     "different_non_semver_less",
			v1:       "alpha",
			v2:       "beta",
			expected: -1, // String comparison fallback (v1 < v2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			assert.Equal(t, tt.expected, result,
				"compareVersions(%s, %s) = %d, expected %d",
				tt.v1, tt.v2, result, tt.expected)
		})
	}
}
