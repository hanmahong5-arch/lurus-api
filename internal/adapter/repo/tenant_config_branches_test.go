package repo

// tenant_config_branches_test.go — deterministic branch coverage for the typed
// tenant-config getters: correct-type success, wrong-type-but-convertible,
// unconvertible (falls back to default), and not-found (default). No Redis or
// DB fault injection needed — each branch is driven by seeded config rows.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
)

func seedConfig(t *testing.T, tenantID, key, value, typ string) {
	t.Helper()
	cfg := &TenantConfig{TenantID: tenantID, ConfigKey: key, ConfigValue: value, ConfigType: typ}
	if err := DB.Create(cfg).Error; err != nil {
		t.Fatalf("seed config %s: %v", key, err)
	}
}

func TestGetTenantConfigTyped_Branches_PG(t *testing.T) {
	SetupTestDB(t)
	tid := "cfg-tenant"

	// int: correct type, convertible-other-type, unconvertible, not-found.
	seedConfig(t, tid, "int.ok", "42", entity.ConfigTypeInt)
	seedConfig(t, tid, "int.asstring", "7", entity.ConfigTypeString) // wrong type but parses
	seedConfig(t, tid, "int.bad", "notnum", entity.ConfigTypeInt)

	if got := GetTenantConfigInt(tid, "int.ok", -1); got != 42 {
		t.Errorf("int.ok = %d, want 42", got)
	}
	if got := GetTenantConfigInt(tid, "int.asstring", -1); got != 7 {
		t.Errorf("int.asstring = %d, want 7", got)
	}
	if got := GetTenantConfigInt(tid, "int.bad", -1); got != -1 {
		t.Errorf("int.bad must fall back to default -1, got %d", got)
	}
	if got := GetTenantConfigInt(tid, "missing", 99); got != 99 {
		t.Errorf("missing int must return default 99, got %d", got)
	}

	// bool
	seedConfig(t, tid, "bool.ok", "true", entity.ConfigTypeBool)
	seedConfig(t, tid, "bool.asstring", "1", entity.ConfigTypeString)
	seedConfig(t, tid, "bool.bad", "maybe", entity.ConfigTypeBool)
	if got := GetTenantConfigBool(tid, "bool.ok", false); !got {
		t.Error("bool.ok = false, want true")
	}
	if got := GetTenantConfigBool(tid, "bool.asstring", false); !got {
		t.Error("bool.asstring = false, want true")
	}
	if got := GetTenantConfigBool(tid, "bool.bad", true); !got {
		t.Error("bool.bad must fall back to default true")
	}
	if got := GetTenantConfigBool(tid, "missing", true); !got {
		t.Error("missing bool must return default true")
	}

	// float
	seedConfig(t, tid, "float.ok", "3.14", entity.ConfigTypeFloat)
	seedConfig(t, tid, "float.asstring", "2.5", entity.ConfigTypeString)
	seedConfig(t, tid, "float.bad", "notfloat", entity.ConfigTypeFloat)
	if got := GetTenantConfigFloat(tid, "float.ok", -1); got != 3.14 {
		t.Errorf("float.ok = %v, want 3.14", got)
	}
	if got := GetTenantConfigFloat(tid, "float.asstring", -1); got != 2.5 {
		t.Errorf("float.asstring = %v, want 2.5", got)
	}
	if got := GetTenantConfigFloat(tid, "float.bad", -1); got != -1 {
		t.Errorf("float.bad must fall back to default -1, got %v", got)
	}
	if got := GetTenantConfigFloat(tid, "missing", 9.9); got != 9.9 {
		t.Errorf("missing float must return default 9.9, got %v", got)
	}

	// string getter + not-found
	seedConfig(t, tid, "str.val", "hello", entity.ConfigTypeString)
	if got := GetTenantConfigValue(tid, "str.val", "def"); got != "hello" {
		t.Errorf("str.val = %q, want hello", got)
	}
	if got := GetTenantConfigValue(tid, "missing", "def"); got != "def" {
		t.Errorf("missing string must return default, got %q", got)
	}

	// GetTenantConfig not-found returns a "config not found" error.
	if _, err := GetTenantConfig(tid, "nope"); err == nil {
		t.Error("GetTenantConfig for missing key must error")
	}
}
