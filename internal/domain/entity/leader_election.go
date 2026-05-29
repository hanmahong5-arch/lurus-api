package entity

// LeaderElectionName is the well-known name of the single lease row that
// guards all HA-sensitive background tasks. One lease (rather than one per
// task) keeps the design minimal — every master-only task gates on the same
// leadership flag.
const LeaderElectionName = "master"

// LeaderLeaseTTLSeconds is how long a runtime lease stays valid without
// renewal. A dead leader is replaced within roughly this window. Shared by the
// boot-time migration gate (repo) and the renewal loop (lifecycle) so a node
// that wins the boot lease keeps leadership seamlessly into its renewal loop.
const LeaderLeaseTTLSeconds int64 = 30

// LeaderBootLeaseTTLSeconds is the TTL for the boot-time migration lease. It is
// deliberately much longer than the runtime TTL so a slow cold-start
// AutoMigrate cannot lapse mid-migration and let another replica begin a
// concurrent (racing) migration. The LeaderManager renews with the shorter
// runtime TTL once it starts, shortening the lease back.
const LeaderBootLeaseTTLSeconds int64 = 300

// LeaderElection is the DB-backed lease that implements HA leader election.
// A process is the leader while it owns the row named LeaderElectionName and
// the lease has not expired. All timestamps are unix-second values.
//
// AcquiredAt records when the lease ROW was first created; RenewedAt records
// the most recent successful (re)acquire or renew, so a holder change is
// observable as a RenewedAt jump together with a HolderId change.
type LeaderElection struct {
	Name       string `json:"name" gorm:"primaryKey;size:64"`
	HolderId   string `json:"holder_id" gorm:"size:128;not null;default:''"`
	AcquiredAt int64  `json:"acquired_at" gorm:"not null;default:0"`
	RenewedAt  int64  `json:"renewed_at" gorm:"not null;default:0"`
	ExpiresAt  int64  `json:"expires_at" gorm:"not null;default:0"`
}

func (LeaderElection) TableName() string {
	return "leader_elections"
}
