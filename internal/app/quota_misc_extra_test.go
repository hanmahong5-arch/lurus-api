package app

// quota_misc_extra_test.go — small reachable branches: the reportQuotaThreshold
// guard clauses (nats disabled / missing identity account / zero consumed) and
// the pure CheckSelfDemotion role-guard.

import (
	"context"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestReportQuotaThreshold_NatsDisabledIsNoop(t *testing.T) {
	// LLM_QUOTA_NATS_ENABLED is unset in the test env => hubnats.Enabled() is
	// false, so this must return immediately without touching the DB or panicking.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("reportQuotaThreshold panicked with nats disabled: %v", r)
		}
	}()
	relayInfo := &relaycommon.RelayInfo{
		UserId:            123,
		IdentityAccountID: 456,
	}
	reportQuotaThreshold(context.Background(), relayInfo, 1000)
}

func TestReportQuotaThreshold_ZeroConsumedGuard(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("reportQuotaThreshold panicked: %v", r)
		}
	}()
	// Even if nats were enabled, consumed<=0 / account<=0 short-circuit.
	reportQuotaThreshold(context.Background(), &relaycommon.RelayInfo{}, 0)
}

func TestCheckSelfDemotion(t *testing.T) {
	cases := []struct {
		name                                  string
		actorId, targetId, actorRole, newRole int
		wantErr                               bool
	}{
		{"self demote lower role", 1, 1, common.RoleAdminUser, common.RoleCommonUser, true},
		{"self same role (no demotion)", 1, 1, common.RoleAdminUser, common.RoleAdminUser, false},
		{"self promote", 1, 1, common.RoleCommonUser, common.RoleAdminUser, false},
		{"different target", 1, 2, common.RoleAdminUser, common.RoleCommonUser, false},
		{"newRole 0 means unset", 1, 1, common.RoleAdminUser, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckSelfDemotion(tc.actorId, tc.targetId, tc.actorRole, tc.newRole)
			if (err != nil) != tc.wantErr {
				t.Errorf("CheckSelfDemotion(%d,%d,%d,%d) err=%v, wantErr=%v",
					tc.actorId, tc.targetId, tc.actorRole, tc.newRole, err, tc.wantErr)
			}
		})
	}
}
