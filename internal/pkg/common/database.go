package common

// Runtime is PostgreSQL-only since 2026-06 (MySQL and the SQLite dev fallback
// were removed; see repo.validateSQLDSN). These flags survive solely for the
// hermetic glebarez SQLite unit-test tier, whose test harnesses save/restore
// them around direct DB injection. Do not branch on them in new runtime
// code — assume PostgreSQL. (Intentionally NOT a formal Deprecated: marker:
// that would trip SA1019 on every existing test-tier save/restore.)
var UsingSQLite = false
var UsingPostgreSQL = false
var UsingMySQL = false
var UsingClickHouse = false
