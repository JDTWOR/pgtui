package app

import (
	"context"
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rebelice/lazypg/internal/app/messages"
	"github.com/rebelice/lazypg/internal/db/metadata"
	"github.com/rebelice/lazypg/internal/models"
)

// loadTree loads the database structure skeleton with object counts (fast).
// Extracted from app.go to keep the main file manageable.
func (a *App) loadTree() tea.Msg {
	ctx := context.Background()

	conn, err := a.connectionManager.GetActive()
	if err != nil {
		return messages.TreeLoadedMsg{Err: fmt.Errorf("no active connection: %w", err)}
	}

	currentDB := conn.Config.Database

	// Build root with database node
	root := models.BuildDatabaseTree([]string{currentDB}, currentDB)

	// Load extensions (usually fast, small number)
	extensions, _ := metadata.ListExtensions(ctx, conn.Pool)

	// Get all schema objects in ONE query
	schemaObjects, err := metadata.GetAllSchemaObjects(ctx, conn.Pool)
	if err != nil {
		return messages.TreeLoadedMsg{Err: fmt.Errorf("failed to load schema objects: %w", err)}
	}

	dbNode := root.FindByID(fmt.Sprintf("db:%s", currentDB))
	if dbNode == nil {
		return messages.TreeLoadedMsg{Root: root}
	}

	// Organize objects by schema -> type -> names
	type funcInfo struct {
		name string
		args string
	}
	type schemaData struct {
		tables           []string
		views            []string
		matViews         []string
		sequences        []string
		functions        []funcInfo
		procedures       []funcInfo
		triggerFunctions []string
		compositeTypes   []string
		enumTypes        []string
		domainTypes      []string
		rangeTypes       []string
	}
	schemaMap := make(map[string]*schemaData)

	for _, obj := range schemaObjects {
		sd, ok := schemaMap[obj.SchemaName]
		if !ok {
			sd = &schemaData{}
			schemaMap[obj.SchemaName] = sd
		}
		switch obj.ObjectType {
		case "table":
			sd.tables = append(sd.tables, obj.ObjectName)
		case "view":
			sd.views = append(sd.views, obj.ObjectName)
		case "matview":
			sd.matViews = append(sd.matViews, obj.ObjectName)
		case "sequence":
			sd.sequences = append(sd.sequences, obj.ObjectName)
		case "function":
			sd.functions = append(sd.functions, funcInfo{name: obj.ObjectName, args: obj.Arguments})
		case "procedure":
			sd.procedures = append(sd.procedures, funcInfo{name: obj.ObjectName, args: obj.Arguments})
		case "trigger_function":
			sd.triggerFunctions = append(sd.triggerFunctions, obj.ObjectName)
		case "composite_type":
			sd.compositeTypes = append(sd.compositeTypes, obj.ObjectName)
		case "enum_type":
			sd.enumTypes = append(sd.enumTypes, obj.ObjectName)
		case "domain_type":
			sd.domainTypes = append(sd.domainTypes, obj.ObjectName)
		case "range_type":
			sd.rangeTypes = append(sd.rangeTypes, obj.ObjectName)
		}
	}

	// Add extensions group
	if len(extensions) > 0 {
		extGroup := models.NewTreeNode(
			fmt.Sprintf("extensions:%s", currentDB),
			models.TreeNodeTypeExtensionGroup,
			fmt.Sprintf("Extensions (%d)", len(extensions)),
		)
		extGroup.Selectable = false
		for _, ext := range extensions {
			extNode := models.NewTreeNode(
				fmt.Sprintf("extension:%s.%s", currentDB, ext.Name),
				models.TreeNodeTypeExtension,
				fmt.Sprintf("%s v%s", ext.Name, ext.Version),
			)
			extNode.Selectable = true
			extNode.Metadata = ext
			extNode.Loaded = true
			extGroup.AddChild(extNode)
		}
		extGroup.Loaded = true
		dbNode.AddChild(extGroup)
	}

	// Build tree with pre-populated object nodes
	// Sort schema names for consistent ordering
	schemaNames := make([]string, 0, len(schemaMap))
	for name := range schemaMap {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)

	for _, schemaName := range schemaNames {
		sd := schemaMap[schemaName]
		schemaNode := models.NewTreeNode(
			fmt.Sprintf("schema:%s.%s", currentDB, schemaName),
			models.TreeNodeTypeSchema,
			schemaName,
		)
		schemaNode.Selectable = true

		// Tables group with actual table nodes
		if len(sd.tables) > 0 {
			tablesGroup := models.NewTreeNode(
				fmt.Sprintf("tables:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeTableGroup,
				fmt.Sprintf("Tables (%d)", len(sd.tables)),
			)
			tablesGroup.Selectable = false
			for _, tableName := range sd.tables {
				tableNode := models.NewTreeNode(
					fmt.Sprintf("table:%s.%s.%s", currentDB, schemaName, tableName),
					models.TreeNodeTypeTable,
					tableName,
				)
				tableNode.Selectable = true
				tableNode.Loaded = false // Columns/indexes still lazy load
				tablesGroup.AddChild(tableNode)
			}
			tablesGroup.Loaded = true // Group has all children
			schemaNode.AddChild(tablesGroup)
		}

		// Views group with actual view nodes
		if len(sd.views) > 0 {
			viewsGroup := models.NewTreeNode(
				fmt.Sprintf("views:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeViewGroup,
				fmt.Sprintf("Views (%d)", len(sd.views)),
			)
			viewsGroup.Selectable = false
			for _, viewName := range sd.views {
				viewNode := models.NewTreeNode(
					fmt.Sprintf("view:%s.%s.%s", currentDB, schemaName, viewName),
					models.TreeNodeTypeView,
					viewName,
				)
				viewNode.Selectable = true
				viewNode.Loaded = true // Views don't have children
				viewsGroup.AddChild(viewNode)
			}
			viewsGroup.Loaded = true
			schemaNode.AddChild(viewsGroup)
		}

		// Materialized Views group with actual matview nodes
		if len(sd.matViews) > 0 {
			matViewsGroup := models.NewTreeNode(
				fmt.Sprintf("matviews:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeMaterializedViewGroup,
				fmt.Sprintf("Materialized Views (%d)", len(sd.matViews)),
			)
			matViewsGroup.Selectable = false
			for _, matViewName := range sd.matViews {
				matViewNode := models.NewTreeNode(
					fmt.Sprintf("matview:%s.%s.%s", currentDB, schemaName, matViewName),
					models.TreeNodeTypeMaterializedView,
					matViewName,
				)
				matViewNode.Selectable = true
				matViewNode.Loaded = true // MatViews don't have children
				matViewsGroup.AddChild(matViewNode)
			}
			matViewsGroup.Loaded = true
			schemaNode.AddChild(matViewsGroup)
		}

		// Functions group with actual function nodes
		if len(sd.functions) > 0 {
			funcsGroup := models.NewTreeNode(
				fmt.Sprintf("functions:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeFunctionGroup,
				fmt.Sprintf("Functions (%d)", len(sd.functions)),
			)
			funcsGroup.Selectable = false
			for _, f := range sd.functions {
				// Label includes arguments for unique identification (e.g., "my_func(integer, text)")
				funcLabel := fmt.Sprintf("%s(%s)", f.name, f.args)
				funcNode := models.NewTreeNode(
					fmt.Sprintf("function:%s.%s.%s", currentDB, schemaName, f.name),
					models.TreeNodeTypeFunction,
					funcLabel,
				)
				funcNode.Selectable = true
				funcNode.Loaded = true // Functions don't have children
				funcsGroup.AddChild(funcNode)
			}
			funcsGroup.Loaded = true
			schemaNode.AddChild(funcsGroup)
		}

		// Procedures group with actual procedure nodes
		if len(sd.procedures) > 0 {
			procsGroup := models.NewTreeNode(
				fmt.Sprintf("procedures:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeProcedureGroup,
				fmt.Sprintf("Procedures (%d)", len(sd.procedures)),
			)
			procsGroup.Selectable = false
			for _, p := range sd.procedures {
				// Label includes arguments for unique identification (e.g., "my_proc(integer, text)")
				procLabel := fmt.Sprintf("%s(%s)", p.name, p.args)
				procNode := models.NewTreeNode(
					fmt.Sprintf("procedure:%s.%s.%s", currentDB, schemaName, p.name),
					models.TreeNodeTypeProcedure,
					procLabel,
				)
				procNode.Selectable = true
				procNode.Loaded = true // Procedures don't have children
				procsGroup.AddChild(procNode)
			}
			procsGroup.Loaded = true
			schemaNode.AddChild(procsGroup)
		}

		// Trigger Functions group with actual trigger function nodes
		if len(sd.triggerFunctions) > 0 {
			trigFuncsGroup := models.NewTreeNode(
				fmt.Sprintf("triggerfuncs:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeTriggerFunctionGroup,
				fmt.Sprintf("Trigger Functions (%d)", len(sd.triggerFunctions)),
			)
			trigFuncsGroup.Selectable = false
			for _, trigFuncName := range sd.triggerFunctions {
				trigFuncNode := models.NewTreeNode(
					fmt.Sprintf("triggerfunction:%s.%s.%s", currentDB, schemaName, trigFuncName),
					models.TreeNodeTypeTriggerFunction,
					trigFuncName,
				)
				trigFuncNode.Selectable = true
				trigFuncNode.Loaded = true // Trigger functions don't have children
				trigFuncsGroup.AddChild(trigFuncNode)
			}
			trigFuncsGroup.Loaded = true
			schemaNode.AddChild(trigFuncsGroup)
		}

		// Sequences group with actual sequence nodes
		if len(sd.sequences) > 0 {
			seqsGroup := models.NewTreeNode(
				fmt.Sprintf("sequences:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeSequenceGroup,
				fmt.Sprintf("Sequences (%d)", len(sd.sequences)),
			)
			seqsGroup.Selectable = false
			for _, seqName := range sd.sequences {
				seqNode := models.NewTreeNode(
					fmt.Sprintf("sequence:%s.%s.%s", currentDB, schemaName, seqName),
					models.TreeNodeTypeSequence,
					seqName,
				)
				seqNode.Selectable = true
				seqNode.Loaded = true // Sequences don't have children
				seqsGroup.AddChild(seqNode)
			}
			seqsGroup.Loaded = true
			schemaNode.AddChild(seqsGroup)
		}

		// Composite Types group with actual composite type nodes
		if len(sd.compositeTypes) > 0 {
			compTypesGroup := models.NewTreeNode(
				fmt.Sprintf("compositetypes:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeCompositeTypeGroup,
				fmt.Sprintf("Composite Types (%d)", len(sd.compositeTypes)),
			)
			compTypesGroup.Selectable = false
			for _, compTypeName := range sd.compositeTypes {
				compTypeNode := models.NewTreeNode(
					fmt.Sprintf("compositetype:%s.%s.%s", currentDB, schemaName, compTypeName),
					models.TreeNodeTypeCompositeType,
					compTypeName,
				)
				compTypeNode.Selectable = true
				compTypeNode.Loaded = true // Composite types don't have children
				compTypesGroup.AddChild(compTypeNode)
			}
			compTypesGroup.Loaded = true
			schemaNode.AddChild(compTypesGroup)
		}

		// Enum Types group with actual enum type nodes
		if len(sd.enumTypes) > 0 {
			enumTypesGroup := models.NewTreeNode(
				fmt.Sprintf("enumtypes:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeEnumTypeGroup,
				fmt.Sprintf("Enum Types (%d)", len(sd.enumTypes)),
			)
			enumTypesGroup.Selectable = false
			for _, enumTypeName := range sd.enumTypes {
				enumTypeNode := models.NewTreeNode(
					fmt.Sprintf("enumtype:%s.%s.%s", currentDB, schemaName, enumTypeName),
					models.TreeNodeTypeEnumType,
					enumTypeName,
				)
				enumTypeNode.Selectable = true
				enumTypeNode.Loaded = true // Enum types don't have children
				enumTypesGroup.AddChild(enumTypeNode)
			}
			enumTypesGroup.Loaded = true
			schemaNode.AddChild(enumTypesGroup)
		}

		// Domain Types group with actual domain type nodes
		if len(sd.domainTypes) > 0 {
			domainTypesGroup := models.NewTreeNode(
				fmt.Sprintf("domaintypes:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeDomainTypeGroup,
				fmt.Sprintf("Domain Types (%d)", len(sd.domainTypes)),
			)
			domainTypesGroup.Selectable = false
			for _, domainTypeName := range sd.domainTypes {
				domainTypeNode := models.NewTreeNode(
					fmt.Sprintf("domaintype:%s.%s.%s", currentDB, schemaName, domainTypeName),
					models.TreeNodeTypeDomainType,
					domainTypeName,
				)
				domainTypeNode.Selectable = true
				domainTypeNode.Loaded = true // Domain types don't have children
				domainTypesGroup.AddChild(domainTypeNode)
			}
			domainTypesGroup.Loaded = true
			schemaNode.AddChild(domainTypesGroup)
		}

		// Range Types group with actual range type nodes
		if len(sd.rangeTypes) > 0 {
			rangeTypesGroup := models.NewTreeNode(
				fmt.Sprintf("rangetypes:%s.%s", currentDB, schemaName),
				models.TreeNodeTypeRangeTypeGroup,
				fmt.Sprintf("Range Types (%d)", len(sd.rangeTypes)),
			)
			rangeTypesGroup.Selectable = false
			for _, rangeTypeName := range sd.rangeTypes {
				rangeTypeNode := models.NewTreeNode(
					fmt.Sprintf("rangetype:%s.%s.%s", currentDB, schemaName, rangeTypeName),
					models.TreeNodeTypeRangeType,
					rangeTypeName,
				)
				rangeTypeNode.Selectable = true
				rangeTypeNode.Loaded = true // Range types don't have children
				rangeTypesGroup.AddChild(rangeTypeNode)
			}
			rangeTypesGroup.Loaded = true
			schemaNode.AddChild(rangeTypesGroup)
		}

		schemaNode.Loaded = true
		dbNode.AddChild(schemaNode)
	}

	return messages.TreeLoadedMsg{Root: root}
}
