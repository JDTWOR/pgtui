package components

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/rebelice/lazypg/internal/ui/theme"
)

// Zone ID for SQL Editor mouse click handling
const ZoneSQLEditor = "sql-editor"

// SQLEditorHeightPreset defines the height presets for the editor
type SQLEditorHeightPreset int

const (
	SQLEditorSmall  SQLEditorHeightPreset = iota // 20% of available height
	SQLEditorMedium                              // 35% of available height
	SQLEditorLarge                               // 50% of available height
)

// ExecuteQueryMsg is sent when a query should be executed
type ExecuteQueryMsg struct {
	SQL string
}

// OpenExternalEditorMsg requests opening an external editor
type OpenExternalEditorMsg struct {
	Content string
}

// ExplainQueryMsg is sent when the user requests EXPLAIN ANALYZE for a query.
type ExplainQueryMsg struct {
	SQL string
}

// ExternalEditorResultMsg contains the result from external editor
type ExternalEditorResultMsg struct {
	Content string
	Error   error
}

// SQLEditor is a multiline SQL editor component
type SQLEditor struct {
	// Content
	lines     []string // Lines of text
	cursorRow int      // Current cursor row (0-indexed)
	cursorCol int      // Current cursor column (0-indexed)

	// Dimensions
	Width  int
	Height int

	// State
	expanded     bool
	heightPreset SQLEditorHeightPreset
	Focused      bool // Whether the editor has focus

	// Theme
	Theme theme.Theme

	// History
	history    []string
	historyIdx int

	// Autocomplete
	completionEngine  *CompletionEngine
	suggestions       []CompletionItem
	selectedSuggestion int
	showSuggestions    bool
	lastCharPos       int // Absolute char position for triggering re-suggest
}

// NewSQLEditor creates a new SQL editor
func NewSQLEditor(th theme.Theme) *SQLEditor {
	return &SQLEditor{
		lines:        []string{""},
		cursorRow:    0,
		cursorCol:    0,
		expanded:     false,
		heightPreset: SQLEditorMedium,
		Theme:        th,
		history:      []string{},
		historyIdx:   -1,
	}
}

// IsExpanded returns whether the editor is expanded
func (e *SQLEditor) IsExpanded() bool {
	return e.expanded
}

// Toggle expands or collapses the editor
func (e *SQLEditor) Toggle() {
	e.expanded = !e.expanded
}

// Expand expands the editor
func (e *SQLEditor) Expand() {
	e.expanded = true
}

// Collapse collapses the editor
func (e *SQLEditor) Collapse() {
	e.expanded = false
}

// GetHeightPreset returns the current height preset
func (e *SQLEditor) GetHeightPreset() SQLEditorHeightPreset {
	return e.heightPreset
}

// IncreaseHeight increases the height preset
func (e *SQLEditor) IncreaseHeight() {
	if e.heightPreset < SQLEditorLarge {
		e.heightPreset++
	}
}

// DecreaseHeight decreases the height preset
func (e *SQLEditor) DecreaseHeight() {
	if e.heightPreset > SQLEditorSmall {
		e.heightPreset--
	}
}

// GetHeightRatio returns the height ratio for the current preset
func (e *SQLEditor) GetHeightRatio() float64 {
	switch e.heightPreset {
	case SQLEditorSmall:
		return 0.20
	case SQLEditorMedium:
		return 0.35
	case SQLEditorLarge:
		return 0.50
	default:
		return 0.35
	}
}

// GetContent returns the full content as a single string
func (e *SQLEditor) GetContent() string {
	return strings.Join(e.lines, "\n")
}

// SetContent sets the editor content
func (e *SQLEditor) SetContent(content string) {
	if content == "" {
		e.lines = []string{""}
	} else {
		e.lines = strings.Split(content, "\n")
	}
	e.cursorRow = len(e.lines) - 1
	e.cursorCol = len(e.lines[e.cursorRow])
}

// Clear clears the editor content
func (e *SQLEditor) Clear() {
	e.lines = []string{""}
	e.cursorRow = 0
	e.cursorCol = 0
}

// GetCollapsedHeight returns the height when collapsed (2 lines + border)
func (e *SQLEditor) GetCollapsedHeight() int {
	return 4 // 2 content lines + 2 border lines
}

// MoveCursorLeft moves cursor left
func (e *SQLEditor) MoveCursorLeft() {
	if e.cursorCol > 0 {
		e.cursorCol--
	} else if e.cursorRow > 0 {
		// Move to end of previous line
		e.cursorRow--
		e.cursorCol = len(e.lines[e.cursorRow])
	}
}

// MoveCursorRight moves cursor right
func (e *SQLEditor) MoveCursorRight() {
	if e.cursorCol < len(e.lines[e.cursorRow]) {
		e.cursorCol++
	} else if e.cursorRow < len(e.lines)-1 {
		// Move to start of next line
		e.cursorRow++
		e.cursorCol = 0
	}
}

// MoveCursorUp moves cursor up
func (e *SQLEditor) MoveCursorUp() {
	if e.cursorRow > 0 {
		e.cursorRow--
		// Clamp column to line length
		if e.cursorCol > len(e.lines[e.cursorRow]) {
			e.cursorCol = len(e.lines[e.cursorRow])
		}
	}
}

// MoveCursorDown moves cursor down
func (e *SQLEditor) MoveCursorDown() {
	if e.cursorRow < len(e.lines)-1 {
		e.cursorRow++
		// Clamp column to line length
		if e.cursorCol > len(e.lines[e.cursorRow]) {
			e.cursorCol = len(e.lines[e.cursorRow])
		}
	}
}

// MoveCursorToLineStart moves cursor to start of line
func (e *SQLEditor) MoveCursorToLineStart() {
	e.cursorCol = 0
}

// MoveCursorToLineEnd moves cursor to end of line
func (e *SQLEditor) MoveCursorToLineEnd() {
	e.cursorCol = len(e.lines[e.cursorRow])
}

// MoveCursorToDocStart moves cursor to start of document
func (e *SQLEditor) MoveCursorToDocStart() {
	e.cursorRow = 0
	e.cursorCol = 0
}

// MoveCursorToDocEnd moves cursor to end of document
func (e *SQLEditor) MoveCursorToDocEnd() {
	e.cursorRow = len(e.lines) - 1
	e.cursorCol = len(e.lines[e.cursorRow])
}

// InsertChar inserts a character at cursor position
func (e *SQLEditor) InsertChar(ch rune) {
	line := e.lines[e.cursorRow]
	// Insert character at cursor position
	newLine := line[:e.cursorCol] + string(ch) + line[e.cursorCol:]
	e.lines[e.cursorRow] = newLine
	e.cursorCol++
}

// InsertNewline inserts a new line at cursor position
func (e *SQLEditor) InsertNewline() {
	line := e.lines[e.cursorRow]
	// Split line at cursor
	before := line[:e.cursorCol]
	after := line[e.cursorCol:]

	e.lines[e.cursorRow] = before
	// Insert new line after current
	newLines := make([]string, len(e.lines)+1)
	copy(newLines[:e.cursorRow+1], e.lines[:e.cursorRow+1])
	newLines[e.cursorRow+1] = after
	copy(newLines[e.cursorRow+2:], e.lines[e.cursorRow+1:])
	e.lines = newLines

	e.cursorRow++
	e.cursorCol = 0
}

// DeleteCharBefore deletes character before cursor (backspace)
func (e *SQLEditor) DeleteCharBefore() {
	if e.cursorCol > 0 {
		// Delete character before cursor
		line := e.lines[e.cursorRow]
		e.lines[e.cursorRow] = line[:e.cursorCol-1] + line[e.cursorCol:]
		e.cursorCol--
	} else if e.cursorRow > 0 {
		// Merge with previous line
		prevLine := e.lines[e.cursorRow-1]
		currLine := e.lines[e.cursorRow]
		e.cursorCol = len(prevLine)
		e.lines[e.cursorRow-1] = prevLine + currLine
		// Remove current line
		e.lines = append(e.lines[:e.cursorRow], e.lines[e.cursorRow+1:]...)
		e.cursorRow--
	}
}

// DeleteCharAfter deletes character after cursor (delete key)
func (e *SQLEditor) DeleteCharAfter() {
	line := e.lines[e.cursorRow]
	if e.cursorCol < len(line) {
		// Delete character at cursor
		e.lines[e.cursorRow] = line[:e.cursorCol] + line[e.cursorCol+1:]
	} else if e.cursorRow < len(e.lines)-1 {
		// Merge with next line
		nextLine := e.lines[e.cursorRow+1]
		e.lines[e.cursorRow] = line + nextLine
		// Remove next line
		e.lines = append(e.lines[:e.cursorRow+1], e.lines[e.cursorRow+2:]...)
	}
}

// SQL keywords for syntax highlighting
var sqlKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true,
	"INSERT": true, "INTO": true, "VALUES": true, "UPDATE": true, "SET": true,
	"DELETE": true, "CREATE": true, "TABLE": true, "DROP": true, "ALTER": true,
	"INDEX": true, "VIEW": true, "JOIN": true, "LEFT": true, "RIGHT": true,
	"INNER": true, "OUTER": true, "FULL": true, "ON": true, "AS": true,
	"ORDER": true, "BY": true, "GROUP": true, "HAVING": true, "LIMIT": true,
	"OFFSET": true, "UNION": true, "ALL": true, "DISTINCT": true, "CASE": true,
	"WHEN": true, "THEN": true, "ELSE": true, "END": true, "NULL": true,
	"NOT": true, "IN": true, "EXISTS": true, "BETWEEN": true, "LIKE": true,
	"IS": true, "TRUE": true, "FALSE": true, "ASC": true, "DESC": true,
	"PRIMARY": true, "KEY": true, "FOREIGN": true, "REFERENCES": true,
	"CONSTRAINT": true, "UNIQUE": true, "CHECK": true, "DEFAULT": true,
	"CASCADE": true, "NULLS": true, "FIRST": true, "LAST": true,
	"BEGIN": true, "COMMIT": true, "ROLLBACK": true, "TRANSACTION": true,
	"WITH": true, "RECURSIVE": true, "RETURNING": true, "COALESCE": true,
	"CAST": true, "COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
}

// TokenType represents the type of a syntax token
type TokenType int

const (
	TokenText TokenType = iota
	TokenKeyword
	TokenString
	TokenNumber
	TokenComment
	TokenOperator
)

// Token represents a syntax-highlighted token
type Token struct {
	Type  TokenType
	Value string
}

// tokenizeLine tokenizes a single line for syntax highlighting
func (e *SQLEditor) tokenizeLine(line string) []Token {
	var tokens []Token
	i := 0

	for i < len(line) {
		// Skip whitespace
		if unicode.IsSpace(rune(line[i])) {
			start := i
			for i < len(line) && unicode.IsSpace(rune(line[i])) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenText, Value: line[start:i]})
			continue
		}

		// Comment (-- to end of line)
		if i+1 < len(line) && line[i:i+2] == "--" {
			tokens = append(tokens, Token{Type: TokenComment, Value: line[i:]})
			break
		}

		// String literal (single quotes)
		if line[i] == '\'' {
			start := i
			i++
			for i < len(line) {
				if line[i] == '\'' {
					if i+1 < len(line) && line[i+1] == '\'' {
						// Escaped quote
						i += 2
					} else {
						i++
						break
					}
				} else {
					i++
				}
			}
			tokens = append(tokens, Token{Type: TokenString, Value: line[start:i]})
			continue
		}

		// Number
		if unicode.IsDigit(rune(line[i])) || (line[i] == '.' && i+1 < len(line) && unicode.IsDigit(rune(line[i+1]))) {
			start := i
			for i < len(line) && (unicode.IsDigit(rune(line[i])) || line[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: line[start:i]})
			continue
		}

		// Identifier or keyword
		if unicode.IsLetter(rune(line[i])) || line[i] == '_' {
			start := i
			for i < len(line) && (unicode.IsLetter(rune(line[i])) || unicode.IsDigit(rune(line[i])) || line[i] == '_') {
				i++
			}
			word := line[start:i]
			if sqlKeywords[strings.ToUpper(word)] {
				tokens = append(tokens, Token{Type: TokenKeyword, Value: word})
			} else {
				tokens = append(tokens, Token{Type: TokenText, Value: word})
			}
			continue
		}

		// Operators
		if strings.ContainsRune("=<>!+-*/%&|^~", rune(line[i])) {
			start := i
			for i < len(line) && strings.ContainsRune("=<>!+-*/%&|^~", rune(line[i])) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenOperator, Value: line[start:i]})
			continue
		}

		// Other single characters (parens, commas, etc.)
		tokens = append(tokens, Token{Type: TokenText, Value: string(line[i])})
		i++
	}

	return tokens
}

// renderTokens renders tokens with syntax highlighting
func (e *SQLEditor) renderTokens(tokens []Token) string {
	var result strings.Builder

	for _, token := range tokens {
		var style lipgloss.Style
		switch token.Type {
		case TokenKeyword:
			style = lipgloss.NewStyle().Foreground(e.Theme.Keyword).Bold(true)
		case TokenString:
			style = lipgloss.NewStyle().Foreground(e.Theme.String)
		case TokenNumber:
			style = lipgloss.NewStyle().Foreground(e.Theme.Number)
		case TokenComment:
			style = lipgloss.NewStyle().Foreground(e.Theme.Comment).Italic(true)
		case TokenOperator:
			style = lipgloss.NewStyle().Foreground(e.Theme.Operator)
		default:
			style = lipgloss.NewStyle().Foreground(e.Theme.Foreground)
		}
		result.WriteString(style.Render(token.Value))
	}

	return result.String()
}

// View renders the SQL editor with optional autocomplete popup.
// The popup replaces the bottom N editor lines IN-PLACE so that the
// total rendered height and scroll position stay constant.
func (e *SQLEditor) View() string {
	// Calculate popup height (if visible), including the leading "\n" separator
	popupHeight := e.popupLines()
	if e.showSuggestions && e.expanded && popupHeight > 0 {
		popupHeight++ // leading "\n" from renderSuggestionPopup
	}

	// Calculate visible lines based on height
	contentHeight := e.Height - 2 // Account for borders
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Determine which lines to show (always based on full contentHeight)
	var visibleLines []string
	var startLine int

	if e.expanded {
		if e.cursorRow >= contentHeight {
			startLine = e.cursorRow - contentHeight + 1
		}
		endLine := startLine + contentHeight
		if endLine > len(e.lines) {
			endLine = len(e.lines)
		}

		for i := startLine; i < endLine; i++ {
			visibleLines = append(visibleLines, e.renderLine(i, i == e.cursorRow))
		}

		for len(visibleLines) < contentHeight {
			visibleLines = append(visibleLines, e.renderEmptyLine(startLine+len(visibleLines)))
		}
	} else {
		for i := 0; i < 2 && i < len(e.lines); i++ {
			visibleLines = append(visibleLines, e.renderLine(i, false))
		}
		for len(visibleLines) < 2 {
			visibleLines = append(visibleLines, e.renderEmptyLine(len(visibleLines)))
		}
	}

	// Replace the bottom N editor lines with the popup, IN PLACE.
	// This keeps total height and scroll position the same.
	if e.showSuggestions && e.expanded && popupHeight > 0 {
		// Truncate the editor lines to make room
		if popupHeight <= len(visibleLines) {
			visibleLines = visibleLines[:len(visibleLines)-popupHeight]
		} else {
			visibleLines = nil
		}
		popupContent := e.renderSuggestionPopup()
		visibleLines = append(visibleLines, popupContent)
	}

	content := strings.Join(visibleLines, "\n")

	// Container style
	borderColor := e.Theme.Border
	if e.Focused || e.expanded {
		borderColor = e.Theme.BorderFocused
	}

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	contentWidth := e.Width - containerStyle.GetHorizontalFrameSize()
	containerStyle = containerStyle.Width(contentWidth)

	return zone.Mark(ZoneSQLEditor, containerStyle.Render(content))
}

// popupLines returns the number of lines the suggestion popup will occupy (excluding the leading "\n" separator).
func (e *SQLEditor) popupLines() int {
	if !e.showSuggestions || len(e.suggestions) == 0 {
		return 0
	}
	// Header + items + footer
	return 1 + len(e.suggestions) + 1 // min 3, max 12
}

// renderSuggestionPopup renders the autocomplete dropdown.
func (e *SQLEditor) renderSuggestionPopup() string {
	if len(e.suggestions) == 0 {
		return ""
	}

	var lines []string

	// Popup header
	headerStyle := lipgloss.NewStyle().
		Foreground(e.Theme.Background).
		Background(e.Theme.Info).
		Bold(true).
		Padding(0, 1)
	lines = append(lines, headerStyle.Render(" Autocomplete "))

	// Popup items
	for i, item := range e.suggestions {
		var style lipgloss.Style
		if i == e.selectedSuggestion {
			// Selected item
			style = lipgloss.NewStyle().
				Foreground(e.Theme.Background).
				Background(e.Theme.Cursor).
				Padding(0, 1)
		} else {
			style = lipgloss.NewStyle().
				Foreground(e.Theme.Foreground).
				Background(e.Theme.Selection).
				Padding(0, 1)
		}

		label := item.Label
		if item.Detail != "" {
			label = fmt.Sprintf("%-20s  %s", item.Label, item.Detail)
		}
		lines = append(lines, style.Render(" "+label+" "))
	}

	// Footer hint
	footerStyle := lipgloss.NewStyle().
		Foreground(e.Theme.Comment).
		Padding(0, 1)
	lines = append(lines, footerStyle.Render(" Tab/Enter accept  ·  Esc dismiss "))

	return "\n" + strings.Join(lines, "\n")
}

// renderLine renders a single line with line number and syntax highlighting
func (e *SQLEditor) renderLine(lineNum int, hasCursor bool) string {
	// Line number
	lineNumWidth := e.getLineNumberWidth()
	lineNumStr := fmt.Sprintf("%*d", lineNumWidth-3, lineNum+1)

	lineNumStyle := lipgloss.NewStyle().Foreground(e.Theme.Metadata)
	sepStyle := lipgloss.NewStyle().Foreground(e.Theme.Border)

	lineNumPart := lineNumStyle.Render(lineNumStr) + sepStyle.Render(" │ ")

	// Line content with syntax highlighting
	line := e.lines[lineNum]
	tokens := e.tokenizeLine(line)
	contentPart := e.renderTokens(tokens)

	// Insert cursor if this line has it
	if hasCursor && e.expanded {
		contentPart = e.insertCursor(line, tokens)
	}

	return lineNumPart + contentPart
}

// renderEmptyLine renders an empty line placeholder
func (e *SQLEditor) renderEmptyLine(lineNum int) string {
	lineNumWidth := e.getLineNumberWidth()
	lineNumStr := fmt.Sprintf("%*s", lineNumWidth-3, "~")

	lineNumStyle := lipgloss.NewStyle().Foreground(e.Theme.Metadata)
	sepStyle := lipgloss.NewStyle().Foreground(e.Theme.Border)

	return lineNumStyle.Render(lineNumStr) + sepStyle.Render(" │ ")
}

// getLineNumberWidth returns the width needed for line numbers
func (e *SQLEditor) getLineNumberWidth() int {
	maxLine := len(e.lines)
	if maxLine < 10 {
		maxLine = 10
	}
	digits := len(fmt.Sprintf("%d", maxLine))
	if digits < 2 {
		digits = 2
	}
	return digits + 3 // digits + space + separator
}

// insertCursor inserts the cursor character into the rendered line
func (e *SQLEditor) insertCursor(line string, tokens []Token) string {
	// Rebuild line with cursor
	var result strings.Builder
	charIdx := 0

	cursorStyle := lipgloss.NewStyle().
		Foreground(e.Theme.Background).
		Background(e.Theme.Cursor)

	for _, token := range tokens {
		var style lipgloss.Style
		switch token.Type {
		case TokenKeyword:
			style = lipgloss.NewStyle().Foreground(e.Theme.Keyword).Bold(true)
		case TokenString:
			style = lipgloss.NewStyle().Foreground(e.Theme.String)
		case TokenNumber:
			style = lipgloss.NewStyle().Foreground(e.Theme.Number)
		case TokenComment:
			style = lipgloss.NewStyle().Foreground(e.Theme.Comment).Italic(true)
		case TokenOperator:
			style = lipgloss.NewStyle().Foreground(e.Theme.Operator)
		default:
			style = lipgloss.NewStyle().Foreground(e.Theme.Foreground)
		}

		for _, ch := range token.Value {
			if charIdx == e.cursorCol {
				result.WriteString(cursorStyle.Render(string(ch)))
			} else {
				result.WriteString(style.Render(string(ch)))
			}
			charIdx++
		}
	}

	// Cursor at end of line
	if e.cursorCol >= charIdx {
		result.WriteString(cursorStyle.Render(" "))
	}

	return result.String()
}

// Update handles keyboard input
func (e *SQLEditor) Update(msg tea.KeyMsg) (*SQLEditor, tea.Cmd) {
	// Handle completion popup navigation first
	if e.showSuggestions {
		switch msg.String() {
		case "up":
			if e.selectedSuggestion > 0 {
				e.selectedSuggestion--
			}
			return e, nil
		case "down":
			if e.selectedSuggestion < len(e.suggestions)-1 {
				e.selectedSuggestion++
			}
			return e, nil
		case "tab", "enter":
			e.acceptSuggestion()
			return e, nil
		case "esc":
			e.showSuggestions = false
			e.suggestions = nil
			return e, nil
		case "left", "right":
			// Dismiss suggestions on horizontal movement
			e.showSuggestions = false
			e.suggestions = nil
		}
	}

	switch msg.String() {
	// Cursor movement
	case "left":
		e.MoveCursorLeft()
		e.showSuggestions = false
	case "right":
		e.MoveCursorRight()
		e.showSuggestions = false
	case "up":
		e.MoveCursorUp()
		e.showSuggestions = false
	case "down":
		e.MoveCursorDown()
		e.showSuggestions = false
	case "home":
		e.MoveCursorToLineStart()
		e.showSuggestions = false
	case "end":
		e.MoveCursorToLineEnd()
		e.showSuggestions = false
	case "ctrl+home":
		e.MoveCursorToDocStart()
		e.showSuggestions = false
	case "ctrl+end":
		e.MoveCursorToDocEnd()
		e.showSuggestions = false

	// Text editing
	case "backspace":
		e.DeleteCharBefore()
		e.triggerSuggestions()
	case "delete":
		e.DeleteCharAfter()
		e.triggerSuggestions()
	case "enter":
		e.InsertNewline()
		e.showSuggestions = false
	case "tab":
		// Insert 4 spaces for tab (if no popup active)
		for i := 0; i < 4; i++ {
			e.InsertChar(' ')
		}
	case "ctrl+u":
		e.Clear()
		e.showSuggestions = false

	// History navigation
	case "ctrl+up":
		e.HistoryPrev()
		e.showSuggestions = false
	case "ctrl+down":
		e.HistoryNext()
		e.showSuggestions = false

	// Execute
	case "ctrl+s":
		sql := e.GetCurrentStatement()
		if sql != "" {
			e.AddToHistory(e.GetContent())
			return e, func() tea.Msg {
				return ExecuteQueryMsg{SQL: sql}
			}
		}

	// External editor
	case "ctrl+o":
		return e, func() tea.Msg {
			return OpenExternalEditorMsg{Content: e.GetContent()}
		}

	// EXPLAIN ANALYZE
	case "ctrl+shift+e":
		sql := e.GetCurrentStatement()
		if sql != "" {
			e.AddToHistory(e.GetContent())
			return e, func() tea.Msg {
				return ExplainQueryMsg{SQL: sql}
			}
		}
		return e, nil

	// Dismiss on escape when no popup
	case "esc":
		e.showSuggestions = false

	default:
		// Handle printable characters and newlines (for paste)
		if len(msg.String()) == 1 {
			ch := rune(msg.String()[0])
			if ch == '\n' || ch == '\r' {
				e.InsertNewline()
			} else if ch >= 32 && ch <= 126 {
				e.InsertChar(ch)
				e.triggerSuggestions()
			}
		} else if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				if r == '\n' || r == '\r' {
					e.InsertNewline()
				} else {
					e.InsertChar(r)
				}
			}
			e.triggerSuggestions()
		}
	}

	return e, nil
}

// SetCompletionEngine sets the autocomplete engine for the editor.
func (e *SQLEditor) SetCompletionEngine(engine *CompletionEngine) {
	e.completionEngine = engine
}

// triggerSuggestions computes autocomplete suggestions based on current cursor position.
func (e *SQLEditor) triggerSuggestions() {
	if e.completionEngine == nil || !e.expanded {
		e.showSuggestions = false
		return
	}

	sql := e.GetContent()
	cursorPos := e.getCursorAbsolutePos()

	suggestions := e.completionEngine.Suggest(sql, cursorPos)
	if len(suggestions) == 0 {
		e.showSuggestions = false
		return
	}

	e.suggestions = suggestions
	e.selectedSuggestion = 0
	e.showSuggestions = true
}

// getCursorAbsolutePos returns the absolute character position of the cursor.
func (e *SQLEditor) getCursorAbsolutePos() int {
	pos := 0
	for row := 0; row < e.cursorRow; row++ {
		pos += len(e.lines[row]) + 1 // +1 for newline
	}
	pos += e.cursorCol
	return pos
}

// acceptSuggestion inserts the currently selected suggestion and closes the popup.
func (e *SQLEditor) acceptSuggestion() {
	if !e.showSuggestions || len(e.suggestions) == 0 {
		return
	}

	item := e.suggestions[e.selectedSuggestion]
	insert := item.InsertText

	// Find the start of the current word to replace
	sql := e.GetContent()
	cursorPos := e.getCursorAbsolutePos()
	before := sql[:cursorPos]

	wordStart := cursorPos
	for wordStart > 0 {
		ch := before[wordStart-1]
		if ch == '.' || ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			wordStart--
		} else {
			break
		}
	}

	// Replace word from wordStart to cursorPos with insert text
	after := sql[cursorPos:]

	// Reconstruct content
	newSQL := sql[:wordStart] + insert + after
	e.SetContent(newSQL)

	// Position cursor after inserted text
	// Recalculate position since SetContent resets it
	e.cursorRow = 0
	e.cursorCol = 0
	// Find the position after the inserted text
	searchPos := wordStart + len(insert)
	e.setCursorToAbsolutePos(searchPos)

	e.showSuggestions = false
	e.suggestions = nil
}

// setCursorToAbsolutePos sets cursor to the given absolute character position.
func (e *SQLEditor) setCursorToAbsolutePos(pos int) {
	if pos <= 0 {
		e.cursorRow = 0
		e.cursorCol = 0
		return
	}
	remaining := pos
	for row := 0; row < len(e.lines); row++ {
		if remaining <= len(e.lines[row]) {
			e.cursorRow = row
			e.cursorCol = remaining
			return
		}
		remaining -= len(e.lines[row]) + 1 // +1 for newline
	}
	// End of document
	e.cursorRow = len(e.lines) - 1
	e.cursorCol = len(e.lines[e.cursorRow])
}

// AddToHistory adds content to history
func (e *SQLEditor) AddToHistory(content string) {
	if content == "" {
		return
	}
	// Avoid duplicates
	if len(e.history) > 0 && e.history[len(e.history)-1] == content {
		return
	}
	e.history = append(e.history, content)
	e.historyIdx = len(e.history)
}

// HistoryPrev navigates to previous history entry
func (e *SQLEditor) HistoryPrev() {
	if len(e.history) == 0 {
		return
	}
	if e.historyIdx > 0 {
		e.historyIdx--
		e.SetContent(e.history[e.historyIdx])
	}
}

// HistoryNext navigates to next history entry
func (e *SQLEditor) HistoryNext() {
	if len(e.history) == 0 {
		return
	}
	if e.historyIdx < len(e.history)-1 {
		e.historyIdx++
		e.SetContent(e.history[e.historyIdx])
	} else {
		e.historyIdx = len(e.history)
		e.Clear()
	}
}

// GetCurrentStatement returns the SQL statement at cursor position
func (e *SQLEditor) GetCurrentStatement() string {
	content := e.GetContent()
	if content == "" {
		return ""
	}

	// Find statement boundaries using semicolons
	statements := splitStatements(content)
	if len(statements) == 0 {
		return strings.TrimSpace(content)
	}

	// Find which statement the cursor is in
	charPos := 0
	for row := 0; row < e.cursorRow; row++ {
		charPos += len(e.lines[row]) + 1 // +1 for newline
	}
	charPos += e.cursorCol

	// Find the statement containing this position
	currentPos := 0
	for _, stmt := range statements {
		stmtLen := len(stmt)
		if charPos >= currentPos && charPos <= currentPos+stmtLen {
			return strings.TrimSpace(stmt)
		}
		currentPos += stmtLen + 1 // +1 for semicolon
	}

	// Return last statement if cursor is at end
	return strings.TrimSpace(statements[len(statements)-1])
}

// splitStatements splits SQL content into individual statements
func splitStatements(content string) []string {
	var statements []string
	var current strings.Builder
	inString := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if ch == '\'' && !inString {
			inString = true
			current.WriteByte(ch)
		} else if ch == '\'' && inString {
			// Check for escaped quote
			if i+1 < len(content) && content[i+1] == '\'' {
				current.WriteByte(ch)
				current.WriteByte(content[i+1])
				i++
			} else {
				inString = false
				current.WriteByte(ch)
			}
		} else if ch == ';' && !inString {
			stmt := current.String()
			if strings.TrimSpace(stmt) != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}

	// Add remaining content
	stmt := current.String()
	if strings.TrimSpace(stmt) != "" {
		statements = append(statements, stmt)
	}

	return statements
}
