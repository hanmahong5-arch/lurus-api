package repo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"gorm.io/gorm"
)

// TryAcquireOrRenew attempts to acquire or renew the named leader lease for
// holderId. ttl and now are unix-second values; now is injected so the
// election semantics are fully deterministic under test. It returns true when
// the caller holds a valid lease after the call.
//
// Concurrency contract: the renew/takeover path is a single conditional
// UPDATE, so the database serializes competing writers — at most one of them
// can match the (holder-is-me OR lease-expired) predicate and win per row. The
// first-ever acquire is an INSERT under the name primary key; a racing loser
// gets a duplicate-key error, which is reported as "not leader" rather than as
// a failure. The net invariant: at any wall-clock instant at most one holder
// owns a non-expired lease for a given name.
func TryAcquireOrRenew(name, holderId string, ttl, now int64) (bool, error) {
	if name == "" || holderId == "" {
		return false, fmt.Errorf("leader election: name and holderId are required")
	}
	if ttl <= 0 {
		return false, fmt.Errorf("leader election: ttl must be positive, got %d", ttl)
	}
	expiresAt := now + ttl

	// Conditional renew/takeover: win iff we already hold it or the current
	// lease has expired. RowsAffected == 1 means we (re)acquired it.
	res := DB.Model(&entity.LeaderElection{}).
		Where("name = ? AND (holder_id = ? OR expires_at < ?)", name, holderId, now).
		Updates(map[string]interface{}{
			"holder_id":  holderId,
			"renewed_at": now,
			"expires_at": expiresAt,
		})
	if res.Error != nil {
		return false, fmt.Errorf("leader election renew: %w", res.Error)
	}
	if res.RowsAffected == 1 {
		return true, nil
	}

	// No row was updated: either the lease row does not exist yet, or another
	// holder owns a still-valid lease. Try to create it; a duplicate-key error
	// means a concurrent writer owns it — the normal "someone else leads" path.
	lease := &entity.LeaderElection{
		Name:       name,
		HolderId:   holderId,
		AcquiredAt: now,
		RenewedAt:  now,
		ExpiresAt:  expiresAt,
	}
	if err := DB.Create(lease).Error; err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, fmt.Errorf("leader election acquire: %w", err)
	}
	return true, nil
}

// ReleaseLease relinquishes the named lease if this holder still owns it, by
// expiring it immediately. A gracefully shutting-down leader calls this so the
// successor takes over at once instead of waiting out the full TTL. It is a
// no-op when another holder already owns the lease.
func ReleaseLease(name, holderId string) error {
	res := DB.Model(&entity.LeaderElection{}).
		Where("name = ? AND holder_id = ?", name, holderId).
		Update("expires_at", 0)
	if res.Error != nil {
		return fmt.Errorf("leader election release: %w", res.Error)
	}
	return nil
}

// isDuplicateKeyError reports whether err is a unique/primary-key violation.
// TranslateError is not enabled on the GORM config, so gorm.ErrDuplicatedKey
// is not reliably returned; fall back to dialect-specific message matching for
// both SQLite (dev/tests) and PostgreSQL (production).
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || // SQLite
		strings.Contains(msg, "duplicate key value") || // PostgreSQL text
		strings.Contains(msg, "23505") // PostgreSQL SQLSTATE
}
