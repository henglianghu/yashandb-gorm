package yasdb

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
)

func registerColumnMappingCallbacks(db *gorm.DB) {
	_ = db.Callback().Query().Before("gorm:query").Register("yasdb:column_mapping", MismatchedCaseHandler)
}

// MismatchedCaseHandler maps uppercase result-set column names back to schema field names.
// YashanDB returns unquoted identifiers in uppercase; GORM scan uses case-sensitive lookup.
func MismatchedCaseHandler(gormDB *gorm.DB) {
	if gormDB.Statement == nil || gormDB.Statement.Schema == nil {
		return
	}
	if isNamingCaseSensitive(gormDB) {
		return
	}
	if len(gormDB.Statement.Schema.Fields) > 0 && gormDB.Statement.ColumnMapping == nil {
		gormDB.Statement.ColumnMapping = map[string]string{}
	}

	for _, field := range gormDB.Statement.Schema.Fields {
		dbName := TryRemoveQuotes(field.DBName)
		gormDB.Statement.ColumnMapping[strings.ToUpper(dbName)] = field.Name
	}

	addJoinColumnMappings(gormDB.Statement)
}

func addJoinColumnMappings(stmt *gorm.Statement) {
	if stmt == nil || stmt.Schema == nil || len(stmt.Joins) == 0 {
		return
	}

	for _, join := range stmt.Joins {
		relations, ok := resolveJoinRelations(stmt.Schema, join.Name)
		if !ok {
			continue
		}

		parentTableName := clause.CurrentTable
		for idx, rel := range relations {
			curAliasName := rel.Name
			if parentTableName != clause.CurrentTable {
				curAliasName = utils.NestedRelationName(parentTableName, curAliasName)
			}

			aliasName := curAliasName
			if idx == len(relations)-1 && join.Alias != "" {
				aliasName = join.Alias
			}

			addNestedFieldMappings(stmt.ColumnMapping, aliasName, rel.FieldSchema)
			parentTableName = curAliasName
		}
	}
}

func resolveJoinRelations(root *schema.Schema, joinName string) ([]*schema.Relationship, bool) {
	if rel, ok := root.Relationships.Relations[joinName]; ok {
		return []*schema.Relationship{rel}, true
	}

	names := strings.Split(joinName, ".")
	if len(names) <= 1 {
		return nil, false
	}

	relations := make([]*schema.Relationship, 0, len(names))
	currentRelations := root.Relationships.Relations
	for _, name := range names {
		rel, ok := currentRelations[name]
		if !ok {
			return nil, false
		}
		relations = append(relations, rel)
		currentRelations = rel.FieldSchema.Relationships.Relations
	}

	return relations, true
}

func addNestedFieldMappings(columnMapping map[string]string, aliasName string, joinSchema *schema.Schema) {
	if len(columnMapping) == 0 || aliasName == "" || joinSchema == nil {
		return
	}

	for _, dbName := range joinSchema.DBNames {
		nestedName := utils.NestedRelationName(aliasName, TryRemoveQuotes(dbName))
		columnMapping[strings.ToUpper(nestedName)] = nestedName
	}
}

func isNamingCaseSensitive(db *gorm.DB) bool {
	if dialector, ok := db.Dialector.(*Dialector); ok && dialector.Config != nil {
		return dialector.NamingCaseSensitive
	}
	if dialector, ok := db.Dialector.(Dialector); ok && dialector.Config != nil {
		return dialector.NamingCaseSensitive
	}
	return false
}
