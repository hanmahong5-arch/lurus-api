package repo

import "testing"

// TestSecuritySQLDSNFastFail_PGOnly locks the PG-only boot gate: any SQL_DSN
// that is not a postgres://|postgresql:// DSN must refuse to boot — including
// the historical SQLite dev fallback (empty DSN, "local") and MySQL DSNs,
// which were valid before the 2026-06 PG-only convergence. This is an
// intentional semantic reversal of the old fast-fail rule that only refused
// an empty DSN inside a container.
func TestSecuritySQLDSNFastFail_PGOnly(t *testing.T) {
	cases := []struct {
		name    string
		sqlDSN  string
		wantErr bool
	}{
		{"empty dsn refused", "", true},
		{"mysql scheme refused", "mysql://u:p@h/db", true},
		{"mysql go-sql-driver dsn refused", "root:pw@tcp(h:3306)/db", true},
		{"sqlite local fallback refused", "local", true},
		{"postgres scheme allowed", "postgres://u:p@h/db", false},
		{"postgresql scheme allowed", "postgresql://u:p@h/db", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSQLDSN(tc.sqlDSN)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateSQLDSN(%q) = %v, wantErr %v", tc.sqlDSN, err, tc.wantErr)
			}
		})
	}
}
