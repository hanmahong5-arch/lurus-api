package repo

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/migration"
	"github.com/LurusTech/lurus-hub/migrations"
)

// TestMigrationBaselineThrough_MatchesEmbeddedHead locks the wiring
// constant against the embedded FS: migrationBaselineThrough must name a
// real embedded file AND be the 20th version in lex order, so the boot
// runner baselines exactly migrations 001–020 and executes only 021+.
// If someone renames/deletes a baseline file or bumps the constant
// without shipping the matching SQL, this fails before boot does.
func TestMigrationBaselineThrough_MatchesEmbeddedHead(t *testing.T) {
	versions, err := migration.DiscoverVersions(migrations.FS)
	if err != nil {
		t.Fatalf("DiscoverVersions(migrations.FS): %v", err)
	}
	if len(versions) < 20 {
		t.Fatalf("embedded migrations = %d files, want >= 20", len(versions))
	}
	if versions[19] != migrationBaselineThrough {
		t.Errorf("versions[19] = %q, want migrationBaselineThrough %q",
			versions[19], migrationBaselineThrough)
	}
}
