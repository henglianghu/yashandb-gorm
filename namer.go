package yasdb

import (
	"strings"

	"gorm.io/gorm/schema"
)

type Namer struct {
	schema.NamingStrategy
}

func ConvertNameToFormat(x string) string {
	return strings.ToUpper(x)
}

func (n Namer) TableName(table string) (name string) {
	return ConvertNameToFormat(n.NamingStrategy.TableName(table))
}

func (n Namer) ColumnName(table, column string) (name string) {
	return ConvertNameToFormat(n.NamingStrategy.ColumnName(table, column))
}

func (n Namer) JoinTableName(table string) (name string) {
	return ConvertNameToFormat(n.NamingStrategy.JoinTableName(table))
}

func (n Namer) RelationshipFKName(relationship schema.Relationship) (name string) {
	return ConvertNameToFormat(n.NamingStrategy.RelationshipFKName(relationship))
}

func (n Namer) CheckerName(table, column string) (name string) {
	return ConvertNameToFormat(n.NamingStrategy.CheckerName(table, column))
}

func (n Namer) IndexName(table, column string) (name string) {
	table = TryRemoveQuotes(table)
	return ConvertNameToFormat(n.NamingStrategy.IndexName(table, column))
}

// NormalizeSchemaColumnNames uppercases schema column names so explicit gorm:"column:..."
// tags stay consistent with YashanDB result set metadata and default Namer output.
func NormalizeSchemaColumnNames(s *schema.Schema) {
	if s == nil || schemaColumnNamesNormalized(s) {
		return
	}

	newFieldsByDBName := make(map[string]*schema.Field, len(s.FieldsByDBName))
	for oldName, field := range s.FieldsByDBName {
		newName := ConvertNameToFormat(oldName)
		field.DBName = newName
		newFieldsByDBName[newName] = field
	}
	s.FieldsByDBName = newFieldsByDBName

	for i, name := range s.DBNames {
		s.DBNames[i] = ConvertNameToFormat(name)
	}
	for i, name := range s.PrimaryFieldDBNames {
		s.PrimaryFieldDBNames[i] = ConvertNameToFormat(name)
	}
}

func schemaColumnNamesNormalized(s *schema.Schema) bool {
	for _, name := range s.DBNames {
		if name != ConvertNameToFormat(name) {
			return false
		}
	}
	return true
}
