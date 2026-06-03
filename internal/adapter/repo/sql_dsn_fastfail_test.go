package repo

import "testing"

// TestSecuritySQLDSNFastFail_Predicate locks the fast-fail rule: a container with no
// SQL_DSN must refuse to boot on the SQLite dev fallback (writes would silently fail
// on a read-only root filesystem), while a non-container dev run or any configured DSN
// is allowed through.
func TestSecuritySQLDSNFastFail_Predicate(t *testing.T) {
	cases := []struct {
		name        string
		sqlDSN      string
		inContainer bool
		want        bool
	}{
		{"empty dsn in container refuses", "", true, true},
		{"empty dsn outside container allowed", "", false, false},
		{"postgres dsn in container allowed", "postgresql://u:p@h/db", true, false},
		{"non-empty dsn in container allowed", "local", true, false},
		{"postgres dsn outside container allowed", "postgresql://u:p@h/db", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRefuseSQLiteFallback(tc.sqlDSN, tc.inContainer); got != tc.want {
				t.Errorf("shouldRefuseSQLiteFallback(%q, %v) = %v, want %v",
					tc.sqlDSN, tc.inContainer, got, tc.want)
			}
		})
	}
}
