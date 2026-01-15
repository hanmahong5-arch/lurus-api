package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Table definitions with their column information
type TableDef struct {
	Name       string
	Columns    []string
	PrimaryKey []string
	HasJSON    []string // columns that contain JSON data
}

var tables = []TableDef{
	{Name: "users", PrimaryKey: []string{"id"}},
	{Name: "channels", PrimaryKey: []string{"id"}, HasJSON: []string{"channel_info"}},
	{Name: "tokens", PrimaryKey: []string{"id"}},
	{Name: "options", PrimaryKey: []string{"key"}},
	{Name: "redemptions", PrimaryKey: []string{"id"}},
	{Name: "abilities", PrimaryKey: []string{"group", "model", "channel_id"}},
	{Name: "logs", PrimaryKey: []string{"id"}},
	{Name: "midjourneys", PrimaryKey: []string{"id"}},
	{Name: "top_ups", PrimaryKey: []string{"id"}},
	{Name: "quota_data", PrimaryKey: []string{"id"}},
	{Name: "tasks", PrimaryKey: []string{"id"}, HasJSON: []string{"properties", "private_data", "data"}},
	{Name: "models", PrimaryKey: []string{"id"}},
	{Name: "vendors", PrimaryKey: []string{"id"}},
	{Name: "prefill_groups", PrimaryKey: []string{"id"}},
	{Name: "setups", PrimaryKey: []string{"id"}},
	{Name: "two_fas", PrimaryKey: []string{"id"}},
	{Name: "two_fa_backup_codes", PrimaryKey: []string{"id"}},
	{Name: "checkins", PrimaryKey: []string{"id"}},
	{Name: "passkey_credentials", PrimaryKey: []string{"id"}},
}

// PostgreSQL reserved words that need quoting
var pgReservedWords = map[string]bool{
	"group": true, "key": true, "user": true, "order": true,
	"table": true, "index": true, "select": true, "from": true,
	"where": true, "and": true, "or": true, "not": true,
}

func quoteIdentifier(name string, isPg bool) string {
	if isPg {
		if pgReservedWords[strings.ToLower(name)] {
			return `"` + name + `"`
		}
		return `"` + name + `"`
	}
	return "`" + name + "`"
}

func main() {
	mysqlDSN := flag.String("mysql", "", "MySQL DSN (user:password@tcp(host:port)/dbname)")
	pgDSN := flag.String("pg", "", "PostgreSQL DSN (postgres://user:password@host:port/dbname)")
	tableFilter := flag.String("tables", "", "Comma-separated list of tables to migrate (empty = all)")
	batchSize := flag.Int("batch", 1000, "Batch size for inserts")
	dryRun := flag.Bool("dry-run", false, "Only show what would be done")
	truncate := flag.Bool("truncate", false, "Truncate target tables before insert")
	flag.Parse()

	if *mysqlDSN == "" || *pgDSN == "" {
		fmt.Println("MySQL to PostgreSQL Migration Tool for new-api")
		fmt.Println("\nUsage:")
		fmt.Println("  migrate -mysql \"user:pass@tcp(host:3306)/db\" -pg \"postgres://user:pass@host:5432/db\"")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nExample:")
		fmt.Println("  migrate -mysql \"root:123456@tcp(localhost:3306)/one_api?parseTime=true\" \\")
		fmt.Println("          -pg \"postgres://postgres:123456@localhost:5432/one_api\" \\")
		fmt.Println("          -batch 500 -truncate")
		os.Exit(1)
	}

	// Connect to MySQL
	log.Println("Connecting to MySQL...")
	mysqlDB, err := sql.Open("mysql", *mysqlDSN)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer mysqlDB.Close()

	if err := mysqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping MySQL: %v", err)
	}
	log.Println("MySQL connected successfully")

	// Connect to PostgreSQL
	log.Println("Connecting to PostgreSQL...")
	pgDB, err := sql.Open("pgx", *pgDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgDB.Close()

	if err := pgDB.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("PostgreSQL connected successfully")

	// Filter tables if specified
	tablesToMigrate := tables
	if *tableFilter != "" {
		filterSet := make(map[string]bool)
		for _, t := range strings.Split(*tableFilter, ",") {
			filterSet[strings.TrimSpace(t)] = true
		}
		var filtered []TableDef
		for _, t := range tables {
			if filterSet[t.Name] {
				filtered = append(filtered, t)
			}
		}
		tablesToMigrate = filtered
	}

	// Migrate each table
	totalRows := 0
	startTime := time.Now()

	for _, table := range tablesToMigrate {
		log.Printf("=== Migrating table: %s ===", table.Name)

		// Get table structure from MySQL
		columns, err := getTableColumns(mysqlDB, table.Name)
		if err != nil {
			log.Printf("Warning: Failed to get columns for %s: %v, skipping...", table.Name, err)
			continue
		}
		table.Columns = columns

		if *dryRun {
			count, _ := getRowCount(mysqlDB, table.Name)
			log.Printf("[DRY-RUN] Would migrate %d rows from %s", count, table.Name)
			continue
		}

		// Truncate if requested
		if *truncate {
			log.Printf("Truncating %s...", table.Name)
			_, err := pgDB.Exec(fmt.Sprintf(`TRUNCATE TABLE %s CASCADE`, quoteIdentifier(table.Name, true)))
			if err != nil {
				log.Printf("Warning: Failed to truncate %s: %v", table.Name, err)
			}
		}

		// Migrate data
		rows, err := migrateTable(mysqlDB, pgDB, table, *batchSize)
		if err != nil {
			log.Printf("Error migrating %s: %v", table.Name, err)
			continue
		}
		totalRows += rows
		log.Printf("Migrated %d rows from %s", rows, table.Name)
	}

	duration := time.Since(startTime)
	log.Printf("=== Migration completed ===")
	log.Printf("Total rows migrated: %d", totalRows)
	log.Printf("Total time: %v", duration)
}

func getTableColumns(db *sql.DB, tableName string) ([]string, error) {
	query := fmt.Sprintf("SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = '%s' AND TABLE_SCHEMA = DATABASE() ORDER BY ORDINAL_POSITION", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func getRowCount(db *sql.DB, tableName string) (int, error) {
	var count int
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&count)
	return count, err
}

func migrateTable(mysqlDB, pgDB *sql.DB, table TableDef, batchSize int) (int, error) {
	// Build SELECT query
	var quotedCols []string
	for _, col := range table.Columns {
		quotedCols = append(quotedCols, quoteIdentifier(col, false))
	}
	selectQuery := fmt.Sprintf("SELECT %s FROM %s", strings.Join(quotedCols, ", "), quoteIdentifier(table.Name, false))

	// Query MySQL
	rows, err := mysqlDB.Query(selectQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to query MySQL: %w", err)
	}
	defer rows.Close()

	// Prepare column info
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return 0, fmt.Errorf("failed to get column types: %w", err)
	}

	// Build INSERT query for PostgreSQL
	var pgQuotedCols []string
	var placeholders []string
	for i, col := range table.Columns {
		pgQuotedCols = append(pgQuotedCols, quoteIdentifier(col, true))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	// Build upsert query (INSERT ... ON CONFLICT DO UPDATE)
	var pkCols []string
	for _, pk := range table.PrimaryKey {
		pkCols = append(pkCols, quoteIdentifier(pk, true))
	}

	var updateCols []string
	for _, col := range table.Columns {
		isPK := false
		for _, pk := range table.PrimaryKey {
			if col == pk {
				isPK = true
				break
			}
		}
		if !isPK {
			updateCols = append(updateCols, fmt.Sprintf("%s = EXCLUDED.%s", quoteIdentifier(col, true), quoteIdentifier(col, true)))
		}
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
		quoteIdentifier(table.Name, true),
		strings.Join(pgQuotedCols, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(pkCols, ", "),
		strings.Join(updateCols, ", "),
	)

	// If no update columns (all columns are PKs), use DO NOTHING
	if len(updateCols) == 0 {
		insertQuery = fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
			quoteIdentifier(table.Name, true),
			strings.Join(pgQuotedCols, ", "),
			strings.Join(placeholders, ", "),
		)
	}

	// Create JSON column set for faster lookup
	jsonCols := make(map[string]bool)
	for _, jc := range table.HasJSON {
		jsonCols[jc] = true
	}

	// Process rows in batches
	totalRows := 0
	batch := make([][]interface{}, 0, batchSize)

	for rows.Next() {
		// Create value holders
		values := make([]interface{}, len(colTypes))
		valuePtrs := make([]interface{}, len(colTypes))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return totalRows, fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert values for PostgreSQL
		convertedValues := make([]interface{}, len(values))
		for i, v := range values {
			convertedValues[i] = convertValue(v, table.Columns[i], jsonCols)
		}

		batch = append(batch, convertedValues)

		if len(batch) >= batchSize {
			if err := executeBatch(pgDB, insertQuery, batch); err != nil {
				return totalRows, fmt.Errorf("failed to execute batch: %w", err)
			}
			totalRows += len(batch)
			batch = batch[:0]
			log.Printf("  Progress: %d rows...", totalRows)
		}
	}

	// Insert remaining rows
	if len(batch) > 0 {
		if err := executeBatch(pgDB, insertQuery, batch); err != nil {
			return totalRows, fmt.Errorf("failed to execute final batch: %w", err)
		}
		totalRows += len(batch)
	}

	return totalRows, nil
}

func executeBatch(db *sql.DB, query string, batch [][]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, values := range batch {
		if _, err := stmt.Exec(values...); err != nil {
			return fmt.Errorf("insert failed: %w (values: %v)", err, values)
		}
	}

	return tx.Commit()
}

func convertValue(v interface{}, colName string, jsonCols map[string]bool) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []byte:
		str := string(val)
		// Check if it's a JSON column
		if jsonCols[colName] {
			// Validate JSON
			var js interface{}
			if json.Unmarshal(val, &js) == nil {
				return str
			}
			// If not valid JSON, return as string
			return str
		}
		return str

	case int64:
		return val

	case float64:
		return val

	case bool:
		return val

	case time.Time:
		return val

	case string:
		return val

	default:
		// Handle unknown types
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			return nil
		}
		return v
	}
}
