package common

// Runtime is PostgreSQL-only since 2026-06 (MySQL and the SQLite dev fallback
// were removed; see repo.validateSQLDSN). These flags survive solely for the
// hermetic glebarez SQLite unit-test tier, whose test harnesses save/restore
// them around direct DB injection.
//
// Deprecated: do not branch on these in new runtime code — assume PostgreSQL.
var UsingSQLite = false
var UsingPostgreSQL = false
var UsingMySQL = false
var UsingClickHouse = false
