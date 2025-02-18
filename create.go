package yasdb

import (
	"database/sql"
	"reflect"

	"github.com/thoas/go-funk"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	gormSchema "gorm.io/gorm/schema"
)

func Create(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	stmt := db.Statement
	if stmt == nil {
		return
	}
	schema := stmt.Schema
	if schema == nil {
		return
	}

	hasDefaultValues := len(schema.FieldsWithDefaultDBValue) > 0
	if !stmt.Unscoped {
		for _, c := range schema.CreateClauses {
			stmt.AddClause(c)
		}
	}
	boundVars := make(map[string]int)

	if stmt.SQL.String() == "" {
		values := callbacks.ConvertToCreateValues(stmt)
		_, hasConflict := stmt.Clauses["ON CONFLICT"].Expression.(clause.OnConflict)
		if hasConflict {
			stmt.AddClauseIfNotExists(clause.Insert{Table: clause.Table{Name: stmt.Table}})
			stmt.AddClause(values)
			db.Statement.Build("INSERT", "VALUES", "ON CONFLICT")
		} else {
			stmt.AddClauseIfNotExists(clause.Insert{Table: clause.Table{Name: stmt.Table}})
			stmt.AddClause(clause.Values{Columns: values.Columns, Values: [][]interface{}{values.Values[0]}})
			if hasDefaultValues {
				stmt.AddClauseIfNotExists(clause.Returning{
					Columns: funk.Map(schema.FieldsWithDefaultDBValue, func(field *gormSchema.Field) clause.Column {
						return clause.Column{Name: field.DBName}
					}).([]clause.Column),
				})
			}
			stmt.Build("INSERT", "VALUES", "RETURNING")
			if hasDefaultValues {
				_, _ = stmt.WriteString(" INTO ")
				for idx, field := range schema.FieldsWithDefaultDBValue {
					if idx > 0 {
						_ = stmt.WriteByte(',')
					}
					boundVars[field.Name] = len(stmt.Vars)
					stmt.AddVar(stmt, sql.Out{Dest: reflect.New(field.FieldType).Interface()})
				}
			}
		}

		if !db.DryRun {
			for idx, vals := range values.Values {
				copy(stmt.Vars, vals)
				// HACK HACK: replace values one by one, assuming its value layout will be the same all the time, i.e. aligned
				switch result, err := stmt.ConnPool.ExecContext(stmt.Context, stmt.SQL.String(), stmt.Vars...); err {
				case nil: // success
					db.RowsAffected, _ = result.RowsAffected()
					if hasDefaultValues && !hasConflict {
						insertTo := stmt.ReflectValue
						switch insertTo.Kind() {
						case reflect.Slice, reflect.Array:
							insertTo = insertTo.Index(idx)
						}

						// if hasDefaultValues {
						// bind returning value back to reflected value in the respective fields
						funk.ForEach(
							funk.Filter(schema.FieldsWithDefaultDBValue, func(field *gormSchema.Field) bool {
								return funk.Contains(boundVars, field.Name)
							}),
							func(field *gormSchema.Field) {
								switch insertTo.Kind() {
								case reflect.Struct:
									if err = field.Set(stmt.Context, insertTo, stmt.Vars[boundVars[field.Name]].(sql.Out).Dest); err != nil {
										_ = db.AddError(err)
									}
								case reflect.Map:
									// todo 设置id的值
								}
							},
						)
						// }
					}

				default: // failure
					_ = db.AddError(err)
				}

			}
		}
	}
}
