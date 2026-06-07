package components

import (
	"sort"
	"strings"
)

// CompletionItem represents a single autocomplete suggestion.
type CompletionItem struct {
	Label     string // Display label (e.g., "users")
	Detail    string // Extra info (e.g., "public.table" or "integer")
	InsertText string // Text to insert when selected
}

// MetadataCache holds database metadata for autocomplete suggestions.
// It is populated by the App after tree loading and passed to the SQLEditor.
type MetadataCache struct {
	// schemaName -> table names
	SchemaTables map[string][]string
	// schema.table -> column names
	TableColumns map[string][]string
	// All table names flat (for prefix search)
	allTables []string
	// All column names flat (for prefix search) — includes "schema.table.column" format
	allQualifiedCols []string
}

// NewMetadataCache creates an empty MetadataCache.
func NewMetadataCache() *MetadataCache {
	return &MetadataCache{
		SchemaTables:   make(map[string][]string),
		TableColumns:   make(map[string][]string),
		allTables:      nil,
		allQualifiedCols: nil,
	}
}

// AddTable registers a table with its schema.
func (mc *MetadataCache) AddTable(schema, table string) {
	key := schema + "." + table
	if _, exists := mc.SchemaTables[schema]; !exists {
		mc.SchemaTables[schema] = []string{}
	}
	// Avoid duplicates
	for _, t := range mc.SchemaTables[schema] {
		if t == table {
			return
		}
	}
	mc.SchemaTables[schema] = append(mc.SchemaTables[schema], table)
	mc.allTables = append(mc.allTables, key)
}

// AddColumn registers a column for a table.
// tableKey should be "schema.table".
func (mc *MetadataCache) AddColumn(tableKey, column string) {
	if _, exists := mc.TableColumns[tableKey]; !exists {
		mc.TableColumns[tableKey] = []string{}
	}
	// Avoid duplicates
	for _, c := range mc.TableColumns[tableKey] {
		if c == column {
			return
		}
	}
	mc.TableColumns[tableKey] = append(mc.TableColumns[tableKey], column)
	mc.allQualifiedCols = append(mc.allQualifiedCols, tableKey+"."+column)
}

// SearchTables returns table names matching the prefix.
// Returns "schema.table" format for qualified matches.
func (mc *MetadataCache) SearchTables(prefix string) []string {
	if mc == nil || prefix == "" {
		return nil
	}
	upper := strings.ToUpper(prefix)
	var results []string
	for _, t := range mc.allTables {
		if strings.Contains(strings.ToUpper(t), upper) {
			results = append(results, t)
		}
	}
	sort.Strings(results)
	if len(results) > 20 {
		results = results[:20]
	}
	return results
}

// GetColumns returns column names for a given table key ("schema.table").
func (mc *MetadataCache) GetColumns(tableKey string) []string {
	if mc == nil {
		return nil
	}
	return mc.TableColumns[tableKey]
}

// SearchColumns returns column names matching the prefix across all tables.
// If tableKey is non-empty, only searches columns of that table.
// Prefix can be in "column" or "table.column" format.
func (mc *MetadataCache) SearchColumns(prefix string, tableKey string) []string {
	if mc == nil || prefix == "" {
		return nil
	}
	upper := strings.ToUpper(prefix)

	if tableKey != "" {
		// Search only within the specified table
		var results []string
		for _, c := range mc.TableColumns[tableKey] {
			if strings.Contains(strings.ToUpper(c), upper) {
				results = append(results, c)
			}
		}
		sort.Strings(results)
		if len(results) > 20 {
			results = results[:20]
		}
		return results
	}

	// Search all qualified columns
	var results []string
	for _, qc := range mc.allQualifiedCols {
		if strings.Contains(strings.ToUpper(qc), upper) {
			results = append(results, qc)
		}
	}
	sort.Strings(results)
	if len(results) > 20 {
		results = results[:20]
	}
	return results
}

// CompletionEngine analyzes the current SQL context and generates suggestions.
type CompletionEngine struct {
	metadata *MetadataCache
	keywords []string
}

// NewCompletionEngine creates a new CompletionEngine with the given metadata.
func NewCompletionEngine(metadata *MetadataCache) *CompletionEngine {
	return &CompletionEngine{
		metadata: metadata,
		keywords: sqlKeywordsList(),
	}
}

// SetMetadata updates the metadata cache (e.g., after reconnecting).
func (ce *CompletionEngine) SetMetadata(mc *MetadataCache) {
	ce.metadata = mc
}

// Suggest generates completion suggestions based on the current SQL context.
// sql: the full SQL text being edited.
// cursorPos: absolute character position of the cursor.
// Returns up to 10 sorted suggestions.
func (ce *CompletionEngine) Suggest(sql string, cursorPos int) []CompletionItem {
	if ce.metadata == nil {
		return nil
	}

	// Extract the word being typed at cursor position
	prefix, beforeCursor := ce.extractPrefix(sql, cursorPos)
	if prefix == "" && beforeCursor == "" {
		return nil
	}

	var suggestions []CompletionItem

	// Check if typing after a "." — suggest columns of that table
	if dotIdx := strings.LastIndex(beforeCursor, "."); dotIdx >= 0 {
		tablePrefix := beforeCursor[:dotIdx]
		colPrefix := beforeCursor[dotIdx+1:]

		// Try to match tablePrefix as a table name
		for _, t := range ce.metadata.allTables {
			upperT := strings.ToUpper(t)
			upperTP := strings.ToUpper(tablePrefix)
			if upperT == upperTP || strings.HasSuffix(upperT, "."+upperTP) {
				// Found the table — suggest its columns
				for _, col := range ce.metadata.TableColumns[t] {
					if colPrefix == "" || strings.HasPrefix(strings.ToUpper(col), strings.ToUpper(colPrefix)) {
						suggestions = append(suggestions, CompletionItem{
							Label:      col,
							Detail:     t,
							InsertText: col,
						})
					}
				}
				break
			}
		}

		// Also match partial table name + dot
		if len(suggestions) == 0 {
			for _, t := range ce.metadata.allTables {
				if strings.Contains(strings.ToUpper(t), strings.ToUpper(tablePrefix)) {
					for _, col := range ce.metadata.TableColumns[t] {
						if colPrefix == "" || strings.HasPrefix(strings.ToUpper(col), strings.ToUpper(colPrefix)) {
							suggestions = append(suggestions, CompletionItem{
								Label:      t + "." + col,
								Detail:     "column",
								InsertText: col,
							})
						}
					}
				}
			}
		}

		if len(suggestions) > 10 {
			suggestions = suggestions[:10]
		}
		return suggestions
	}

	// Suggest SQL keywords
	for _, kw := range ce.keywords {
		if strings.HasPrefix(strings.ToUpper(kw), strings.ToUpper(prefix)) {
			suggestions = append(suggestions, CompletionItem{
				Label:      strings.ToUpper(kw),
				Detail:     "keyword",
				InsertText: strings.ToUpper(kw) + " ",
			})
		}
	}

	// Suggest table names
	for _, t := range ce.metadata.allTables {
		if strings.Contains(strings.ToUpper(t), strings.ToUpper(prefix)) {
			// Show just the table name, but insert "schema.table" or just "table"
			parts := strings.SplitN(t, ".", 2)
			shortName := t
			if len(parts) == 2 {
				shortName = parts[1]
			}
			suggestions = append(suggestions, CompletionItem{
				Label:      shortName,
				Detail:     "table · " + t,
				InsertText: shortName,
			})
		}
	}

	// Suggest column names (qualified)
	for _, qc := range ce.metadata.allQualifiedCols {
		if strings.Contains(strings.ToUpper(qc), strings.ToUpper(prefix)) {
			parts := strings.SplitN(qc, ".", 3)
			shortName := qc
			if len(parts) == 3 {
				shortName = parts[2] // Just column name
			}
			suggestions = append(suggestions, CompletionItem{
				Label:      shortName,
				Detail:     "column · " + qc,
				InsertText: shortName,
			})
		}
	}

	// Sort: exact prefix matches first, then alphabetical
	sort.Slice(suggestions, func(i, j int) bool {
		iExact := strings.EqualFold(suggestions[i].Label[:min(len(suggestions[i].Label), len(prefix))], prefix)
		jExact := strings.EqualFold(suggestions[j].Label[:min(len(suggestions[j].Label), len(prefix))], prefix)
		if iExact != jExact {
			return iExact
		}
		return strings.ToLower(suggestions[i].Label) < strings.ToLower(suggestions[j].Label)
	})

	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	return suggestions
}

// extractPrefix extracts the word being typed at the cursor position.
// Returns (prefix, textBeforeCursor) where textBeforeCursor is everything
// from the start of the current word to the cursor.
func (ce *CompletionEngine) extractPrefix(sql string, cursorPos int) (string, string) {
	if cursorPos <= 0 || cursorPos > len(sql) {
		return "", ""
	}

	// Get text before cursor on the current "word"
	before := sql[:cursorPos]

	// Find the start of the current word
	wordStart := cursorPos
	for wordStart > 0 {
		ch := before[wordStart-1]
		if ch == '.' || ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			wordStart--
		} else {
			break
		}
	}

	prefix := before[wordStart:cursorPos]

	// Don't suggest for single characters (too noisy)
	if len(prefix) <= 1 && !strings.Contains(before, ".") {
		return "", ""
	}

	// Get the part before the dot if we're after one
	beforeWord := ""
	if dotIdx := strings.LastIndex(before[:wordStart], "."); dotIdx >= 0 {
		// Include table name before the dot
		tableStart := dotIdx
		for tableStart > 0 {
			ch := before[tableStart-1]
			if ch == '.' || ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
				tableStart--
			} else {
				break
			}
		}
		beforeWord = before[tableStart:wordStart]
	} else {
		beforeWord = before[wordStart:cursorPos]
	}

	return prefix, beforeWord
}

// sqlKeywordsList returns the list of SQL keywords for autocomplete.
func sqlKeywordsList() []string {
	return []string{
		"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "EXISTS",
		"INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE",
		"CREATE", "TABLE", "DROP", "ALTER", "ADD", "COLUMN",
		"INDEX", "VIEW", "MATERIALIZED", "TRIGGER", "FUNCTION",
		"JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS", "ON",
		"AS", "ORDER", "BY", "GROUP", "HAVING",
		"LIMIT", "OFFSET", "UNION", "ALL", "DISTINCT",
		"CASE", "WHEN", "THEN", "ELSE", "END",
		"NULL", "TRUE", "FALSE",
		"BETWEEN", "LIKE", "ILIKE", "SIMILAR", "TO",
		"IS", "NULLS", "FIRST", "LAST",
		"ASC", "DESC",
		"PRIMARY", "KEY", "FOREIGN", "REFERENCES",
		"CONSTRAINT", "UNIQUE", "CHECK", "DEFAULT", "CASCADE",
		"BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION",
		"WITH", "RECURSIVE", "RETURNING",
		"COALESCE", "CAST",
		"COUNT", "SUM", "AVG", "MIN", "MAX",
		"EXPLAIN", "ANALYZE",
		"TRUNCATE", "REFRESH",
		"GRANT", "REVOKE",
		"COPY",
		"IF", "THEN", "ELSE",
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
