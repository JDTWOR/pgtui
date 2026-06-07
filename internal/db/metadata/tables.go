package metadata

import (
	"context"
	"fmt"

	"github.com/rebelice/lazypg/internal/db/connection"
)

// toString safely converts an interface{} to string
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// Schema represents a PostgreSQL schema
type Schema struct {
	Name  string
	Owner string
}

// Table represents a PostgreSQL table
type Table struct {
	Schema   string
	Name     string
	RowCount int64
	Size     string
}

// View represents a PostgreSQL view
type View struct {
	Schema string
	Name   string
}

// ListSchemas returns all schemas in the current database
func ListSchemas(ctx context.Context, pool *connection.Pool) ([]Schema, error) {
	query := `
		SELECT
			schema_name as name,
			schema_owner as owner
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		  AND schema_name NOT LIKE 'pg_temp_%'
		  AND schema_name NOT LIKE 'pg_toast_temp_%'
		ORDER BY schema_name;
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	schemas := make([]Schema, 0, len(rows))
	for _, row := range rows {
		schemas = append(schemas, Schema{
			Name:  toString(row["name"]),
			Owner: toString(row["owner"]),
		})
	}

	return schemas, nil
}

// ListTables returns all tables in a schema
func ListTables(ctx context.Context, pool *connection.Pool, schema string) ([]Table, error) {
	query := `
		SELECT
			schemaname as schema,
			tablename as name,
			pg_catalog.pg_size_pretty(pg_catalog.pg_total_relation_size(schemaname||'.'||tablename)) as size
		FROM pg_catalog.pg_tables
		WHERE schemaname = $1
		ORDER BY tablename;
	`

	rows, err := pool.Query(ctx, query, schema)
	if err != nil {
		return nil, err
	}

	tables := make([]Table, 0, len(rows))
	for _, row := range rows {
		tables = append(tables, Table{
			Schema:   toString(row["schema"]),
			Name:     toString(row["name"]),
			RowCount: 0, // Not populated by ListTables - use GetTableRowCount for specific tables
			Size:     toString(row["size"]),
		})
	}

	return tables, nil
}

// ListViews returns all views in a schema
func ListViews(ctx context.Context, pool *connection.Pool, schema string) ([]View, error) {
	query := `
		SELECT
			schemaname as schema,
			viewname as name
		FROM pg_catalog.pg_views
		WHERE schemaname = $1
		ORDER BY viewname;
	`

	rows, err := pool.Query(ctx, query, schema)
	if err != nil {
		return nil, err
	}

	views := make([]View, 0, len(rows))
	for _, row := range rows {
		views = append(views, View{
			Schema: toString(row["schema"]),
			Name:   toString(row["name"]),
		})
	}

	return views, nil
}

// GetAllTableRowCounts returns estimated row counts for all user tables across all schemas.
// Uses pg_stat_user_tables for fast estimates (no full COUNT(*) scan).
func GetAllTableRowCounts(ctx context.Context, pool *connection.Pool) (map[string]int64, error) {
	query := `
		SELECT schemaname AS schema_name,
			   relname AS table_name,
			   n_live_tup::bigint AS row_count
		FROM pg_stat_user_tables
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table row counts: %w", err)
	}

	counts := make(map[string]int64)
	for _, row := range rows {
		schema := toString(row["schema_name"])
		table := toString(row["table_name"])
		count, ok := row["row_count"].(int64)
		if ok {
			key := schema + "." + table
			counts[key] = count
		}
	}
	return counts, nil
}

// GetTableRowCount returns the estimated row count for a table
func GetTableRowCount(ctx context.Context, pool *connection.Pool, schema, table string) (int64, error) {
	query := `
		SELECT reltuples::bigint as estimate
		FROM pg_class
		WHERE oid = ($1 || '.' || $2)::regclass;
	`

	row, err := pool.QueryRow(ctx, query, schema, table)
	if err != nil {
		return 0, err
	}

	estimate, ok := row["estimate"].(int64)
	if !ok {
		return 0, fmt.Errorf("invalid row count estimate type: %T", row["estimate"])
	}

	return estimate, nil
}
