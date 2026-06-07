package delegates

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rebelice/lazypg/internal/app/messages"
	"github.com/rebelice/lazypg/internal/models"
	"github.com/rebelice/lazypg/internal/ui/components"
)

// Pre-compiled regex patterns for DML statement detection.
// These extract the first table reference from INSERT, UPDATE, DELETE,
// TRUNCATE, and REFRESH MATERIALIZED VIEW statements.
var (
	dmlInsertRe  = regexp.MustCompile(`(?i)\bINSERT\s+INTO\s+([a-zA-Z_][a-zA-Z0-9_.]*)`)
	dmlUpdateRe  = regexp.MustCompile(`(?i)\bUPDATE\s+([a-zA-Z_][a-zA-Z0-9_.]*)`)
	dmlDeleteRe  = regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+([a-zA-Z_][a-zA-Z0-9_.]*)`)
	dmlTruncateRe = regexp.MustCompile(`(?i)\bTRUNCATE(?:\s+TABLE)?\s+([a-zA-Z_][a-zA-Z0-9_.]*)`)
	dmlRefreshMvRe = regexp.MustCompile(`(?i)\bREFRESH\s+MATERIALIZED\s+(?:VIEW\s+)?([a-zA-Z_][a-zA-Z0-9_.]*)`)
)

// QueryDelegate handles query execution and result messages.
type QueryDelegate struct{}

// NewQueryDelegate creates a new QueryDelegate.
func NewQueryDelegate() *QueryDelegate {
	return &QueryDelegate{}
}

// Name returns the delegate name.
func (d *QueryDelegate) Name() string {
	return "query"
}

// Update processes query-related messages.
func (d *QueryDelegate) Update(msg tea.Msg, app AppAccess) (bool, tea.Cmd) {
	switch msg := msg.(type) {

	case components.ExecuteQueryMsg:
		return d.handleExecuteQuery(msg, app)

	case messages.QueryResultMsg:
		return d.handleQueryResult(msg, app)

	case components.SaveObjectMsg:
		return d.handleSaveObject(msg, app)

	case components.ObjectSavedMsg:
		return d.handleObjectSaved(msg, app)
	}

	return false, nil
}

// handleExecuteQuery handles query execution from SQL editor.
func (d *QueryDelegate) handleExecuteQuery(msg components.ExecuteQueryMsg, app AppAccess) (bool, tea.Cmd) {
	if app.GetState().ActiveConnection == nil {
		app.ShowError("No Connection", "Please connect to a database first")
		return true, nil
	}

	// Create pending tab immediately
	app.StartPendingQuery(msg.SQL)

	// Immediately switch focus to data panel and collapse editor
	app.GetSQLEditor().Collapse()
	app.SetFocusArea(models.FocusDataPanel)
	app.UpdatePanelStyles()

	// Execute query asynchronously and start spinner
	return true, tea.Batch(
		app.GetSpinnerTickCmd(),
		app.ExecuteQuery(msg.SQL),
	)
}

// handleQueryResult handles query execution result.
func (d *QueryDelegate) handleQueryResult(msg messages.QueryResultMsg, app AppAccess) (bool, tea.Cmd) {
	// Clear execution cancel function
	app.SetExecuteCancelFn(nil)

	// Handle query result
	if msg.Result.Error != nil {
		// Check if it was cancelled (context cancelled error)
		if msg.Result.Error.Error() == "context canceled" {
			// Already handled by CancelPendingQuery, just return
			return true, nil
		}
		// Show error and remove pending tab
		app.CancelPendingQuery()
		app.ShowError("Query Error", msg.Result.Error.Error())
		return true, nil
	}

	// Record query to history store for later review
	app.RecordQueryHistory(msg.SQL, msg.Result)

	// DML queries (no columns returned): remove the pending tab, show a
	// success toast, refresh affected table tabs, and navigate to the
	// matching table tab (or tree if none exists).
	if len(msg.Result.Columns) == 0 {
		// Remove the pending tab entirely (no "Cancelled" state)
		app.RemovePendingQuery()

		// Show a brief success toast with operation details
		app.ShowSuccessToast(d.formatDMLSummary(msg.SQL, msg.Result))

		// Refresh any affected table data tabs in background
		cmds := d.refreshTabsForDML(msg.SQL, app)

		// Try to navigate to an affected table tab; fall back to tree
		d.navigateToAffectedTable(msg.SQL, app)

		if cmds != nil {
			return true, cmds
		}
		return true, nil
	}

	// SELECT queries: show a proper result tab
	app.CompletePendingQuery(msg.SQL, msg.Result)

	if cmds := d.refreshTabsForDML(msg.SQL, app); cmds != nil {
		return true, cmds
	}

	return true, nil
}

// extractAffectedTableNames parses DML SQL and returns referenced table names.
// Returns both schema-qualified ("schema.table") and simple ("table") names.
func (d *QueryDelegate) extractAffectedTableNames(sql string) []string {
	patterns := []*regexp.Regexp{
		dmlInsertRe,
		dmlUpdateRe,
		dmlDeleteRe,
		dmlTruncateRe,
		dmlRefreshMvRe,
	}

	seen := make(map[string]bool)
	var tables []string

	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(sql, -1)
		for _, m := range matches {
			if len(m) > 1 {
				name := m[1]
				if !seen[name] {
					tables = append(tables, name)
					seen[name] = true
				}
			}
		}
	}

	return tables
}

// refreshTabsForDML looks for existing TabTypeTableData tabs affected by the
// given SQL and returns a batch command to refresh them. Returns nil if no
// DML was detected or no matching tabs exist.
//
// For schema-qualified names ("schema.table") it matches directly against
// tab ObjectIDs. For simple names ("table") it matches any tab whose
// ObjectID ends with ".table".
func (d *QueryDelegate) refreshTabsForDML(sql string, app AppAccess) tea.Cmd {
	names := d.extractAffectedTableNames(sql)
	if len(names) == 0 {
		return nil
	}

	resultTabs := app.GetResultTabs()
	var cmds []tea.Cmd

	for _, name := range names {
		if strings.Contains(name, ".") {
			// Schema-qualified: match directly by ObjectID
			if tab := resultTabs.GetTabByObjectID(name); tab != nil && tab.Type == components.TabTypeTableData {
				if cmd := app.RefreshTableDataTab(name); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		} else {
			// Simple table name: match any tab whose ObjectID ends with ".<name>"
			for _, tab := range resultTabs.GetAllTabs() {
				if tab.Type == components.TabTypeTableData && strings.HasSuffix(tab.ObjectID, "."+name) {
					if cmd := app.RefreshTableDataTab(tab.ObjectID); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// navigateToAffectedTable looks for an existing TabTypeTableData tab matching
// the DML's affected table and switches focus to it. Falls back to tree view.
func (d *QueryDelegate) navigateToAffectedTable(sql string, app AppAccess) {
	names := d.extractAffectedTableNames(sql)
	if len(names) == 0 {
		app.SetFocusArea(models.FocusTreeView)
		app.UpdatePanelStyles()
		return
	}

	resultTabs := app.GetResultTabs()
	for _, name := range names {
		var objectID string
		if strings.Contains(name, ".") {
			objectID = name
		} else {
			// Try to match a tab whose ObjectID ends with ".<name>"
			for _, tab := range resultTabs.GetAllTabs() {
				if tab.Type == components.TabTypeTableData && strings.HasSuffix(tab.ObjectID, "."+name) {
					objectID = tab.ObjectID
					break
				}
			}
		}

		if objectID != "" {
			for i, tab := range resultTabs.GetAllTabs() {
				if tab.ObjectID == objectID && tab.Type == components.TabTypeTableData {
					resultTabs.SetActiveTab(i)
					app.SetFocusArea(models.FocusDataPanel)
					app.UpdatePanelStyles()
					return
				}
			}
		}
	}

	// No matching tab found — focus tree
	app.SetFocusArea(models.FocusTreeView)
	app.UpdatePanelStyles()
}

// formatDMLSummary produces a concise one-line summary of a DML result.
// Example: "INSERT public.users · 3 rows (0.015s)"
func (d *QueryDelegate) formatDMLSummary(sql string, result models.QueryResult) string {
	op := "QUERY"
	if dmlInsertRe.MatchString(sql) {
		op = "INSERT"
	} else if dmlUpdateRe.MatchString(sql) {
		op = "UPDATE"
	} else if dmlDeleteRe.MatchString(sql) {
		op = "DELETE"
	} else if dmlTruncateRe.MatchString(sql) {
		op = "TRUNCATE"
	} else if dmlRefreshMvRe.MatchString(sql) {
		op = "REFRESH MV"
	}

	names := d.extractAffectedTableNames(sql)
	tableName := ""
	if len(names) > 0 {
		tableName = names[0]
	}

	rowStr := fmt.Sprintf("%d rows", result.RowsAffected)
	if result.RowsAffected == 1 {
		rowStr = "1 row"
	}
	durStr := fmt.Sprintf("%.3fs", result.Duration.Seconds())

	if tableName != "" {
		return fmt.Sprintf("%s %s · %s (%s)", op, tableName, rowStr, durStr)
	}
	return fmt.Sprintf("%s · %s (%s)", op, rowStr, durStr)
}

// handleSaveObject handles object definition save request.
func (d *QueryDelegate) handleSaveObject(msg components.SaveObjectMsg, app AppAccess) (bool, tea.Cmd) {
	return true, app.SaveObjectDefinition(msg)
}

// handleObjectSaved handles object save completion.
func (d *QueryDelegate) handleObjectSaved(msg components.ObjectSavedMsg, app AppAccess) (bool, tea.Cmd) {
	if msg.Error != nil {
		app.ShowError("Save Error", fmt.Sprintf("Failed to save object:\n\n%v", msg.Error))
		return true, nil
	}

	// Success - update the code editor's original content and exit edit mode
	resultTabs := app.GetResultTabs()
	activeTab := resultTabs.GetActiveTab()
	if activeTab != nil && activeTab.Type == components.TabTypeCodeEditor && activeTab.CodeEditor != nil {
		activeTab.CodeEditor.Original = activeTab.CodeEditor.GetContent()
		activeTab.CodeEditor.Modified = false
		activeTab.CodeEditor.ExitEditMode(false) // Keep changes
	}

	// Legacy: also update global code editor
	codeEditor := app.GetCodeEditor()
	if codeEditor != nil {
		codeEditor.Original = codeEditor.GetContent()
		codeEditor.Modified = false
		codeEditor.ExitEditMode(false) // Keep changes
	}

	return true, nil
}
