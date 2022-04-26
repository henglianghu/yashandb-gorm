package yasdb

import (
    "database/sql"
    "fmt"
    "regexp"
    "strconv"
    "strings"

    "cod-git.sics.com/cod-noah/gorm-yasdb/clauses"

    _ "cod-git.sics.com/cod-noah/yasdb-go"
    "github.com/thoas/go-funk"
    "gorm.io/gorm"
    "gorm.io/gorm/callbacks"
    "gorm.io/gorm/clause"
    "gorm.io/gorm/logger"
    "gorm.io/gorm/migrator"
    "gorm.io/gorm/schema"
)

const (
    JSON schema.DataType = "json"
)

type Config struct {
    DriverName        string
    DSN               string
    Conn              *sql.DB
    DefaultStringSize uint
}

type Dialector struct {
    *Config
}

func Open(dsn string) gorm.Dialector {
    return &Dialector{Config: &Config{DSN: dsn}}
}

func New(config Config) gorm.Dialector {
    return &Dialector{Config: &config}
}

func SeqName(table string) string {
    return fmt.Sprintf("sequence__%s_", table)
}

func (d Dialector) DummyTableName() string {
    return "DUAL"
}

func (d Dialector) Name() string {
    return "yasdb"
}

func (d Dialector) Initialize(db *gorm.DB) (err error) {
    d.DefaultStringSize = 255

    // register callbacks
    //callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{WithReturning: true})
    callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{LastInsertIDReversed: false})

    d.DriverName = "yasdb"

    if d.Conn != nil {
        db.ConnPool = d.Conn
    } else {
        db.ConnPool, _ = sql.Open(d.DriverName, d.DSN)
    }
    for k, v := range d.ClauseBuilders() {
        db.ClauseBuilders[k] = v
    }
    return
}

func (d Dialector) ClauseBuilders() map[string]clause.ClauseBuilder {
    return map[string]clause.ClauseBuilder{
        "LIMIT": d.RewriteLimit,
        "WHERE": d.RewriteWhere,
    }
}

func (d Dialector) RewriteWhere(c clause.Clause, builder clause.Builder) {
    if where, ok := c.Expression.(clause.Where); ok {
        builder.WriteString(" WHERE ")

        // Switch position if the first query expression is a single Or condition
        for idx, expr := range where.Exprs {
            if v, ok := expr.(clause.OrConditions); !ok || len(v.Exprs) > 1 {
                if idx != 0 {
                    where.Exprs[0], where.Exprs[idx] = where.Exprs[idx], where.Exprs[0]
                }
                break
            }
        }

        wrapInParentheses := false
        for idx, expr := range where.Exprs {
            if idx > 0 {
                if v, ok := expr.(clause.OrConditions); ok && len(v.Exprs) == 1 {
                    builder.WriteString(" OR ")
                } else {
                    builder.WriteString(" AND ")
                }
            }

            if len(where.Exprs) > 1 {
                switch v := expr.(type) {
                case clause.OrConditions:
                    if len(v.Exprs) == 1 {
                        if e, ok := v.Exprs[0].(clause.Expr); ok {
                            sql := strings.ToLower(e.SQL)
                            wrapInParentheses = strings.Contains(sql, "and") || strings.Contains(sql, "or")
                        }
                    }
                case clause.AndConditions:
                    if len(v.Exprs) == 1 {
                        if e, ok := v.Exprs[0].(clause.Expr); ok {
                            sql := strings.ToLower(e.SQL)
                            wrapInParentheses = strings.Contains(sql, "and") || strings.Contains(sql, "or")
                        }
                    }
                case clause.Expr:
                    sql := strings.ToLower(v.SQL)
                    wrapInParentheses = strings.Contains(sql, "and") || strings.Contains(sql, "or")
                }
            }

            if wrapInParentheses {
                builder.WriteString(`(`)
                expr.Build(builder)
                builder.WriteString(`)`)
                wrapInParentheses = false
            } else {
                if e, ok := expr.(clause.IN); ok {
                    if values, ok := e.Values[0].([]interface{}); ok {
                        if len(values) > 1 {
                            newExpr := clauses.IN{
                                Column: expr.(clause.IN).Column,
                                Values: expr.(clause.IN).Values,
                            }
                            newExpr.Build(builder)
                            continue
                        }
                    }
                }

                expr.Build(builder)
            }
        }
    }
}

func (d Dialector) RewriteLimit(c clause.Clause, builder clause.Builder) {
    if limit, ok := c.Expression.(clause.Limit); ok {
        if stmt, ok := builder.(*gorm.Statement); ok {
            if _, ok := stmt.Clauses["ORDER BY"]; !ok {
                s := stmt.Schema
                builder.WriteString("ORDER BY ")
                if s != nil && s.PrioritizedPrimaryField != nil {
                    builder.WriteQuoted(s.PrioritizedPrimaryField.DBName)
                    builder.WriteByte(' ')
                } else {
                    builder.WriteString("(SELECT NULL FROM ")
                    builder.WriteString(d.DummyTableName())
                    builder.WriteString(")")
                }
            }
        }
        if limit := limit.Limit; limit > 0 {
            builder.WriteString(" LIMIT ")
            builder.WriteString(strconv.Itoa(limit))
        }
        if offset := limit.Offset; offset > 0 {
            builder.WriteString(" OFFSET ")
            builder.WriteString(strconv.Itoa(offset))
        }
    }
}

func (d Dialector) DefaultValueOf(*schema.Field) clause.Expression {
    return clause.Expr{SQL: "VALUES (DEFAULT)"}
}

func (d Dialector) Migrator(db *gorm.DB) gorm.Migrator {
    return Migrator{
        Migrator: migrator.Migrator{
            Config: migrator.Config{
                DB:                          db,
                Dialector:                   d,
                CreateIndexAfterCreateTable: true,
            },
        },
    }
}

func (d Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
    writer.WriteString(":")
    writer.WriteString(strconv.Itoa(len(stmt.Vars)))
}

func (d Dialector) QuoteTo(writer clause.Writer, str string) {
    writer.WriteString(str)
}

var numericPlaceholder = regexp.MustCompile(`:(\d+)`)

func (d Dialector) Explain(sql string, vars ...interface{}) string {
    return logger.ExplainSQL(sql, numericPlaceholder, `'`, funk.Map(vars, func(v interface{}) interface{} {
        switch v := v.(type) {
        case bool:
            if v {
                return 1
            }
            return 0
        default:
            return v
        }
    }).([]interface{})...)
}

func (d Dialector) DataTypeOf(field *schema.Field) string {
    delete(field.TagSettings, "RESTRICT")

    var sqlType string

    addStringDefault := func() string {
        var defaultStr string
        if value, ok := field.TagSettings["DEFAULT"]; ok {
            if value == "''" {
                field.DefaultValue = ""
                field.DefaultValueInterface = nil
            } else {
                field.NotNull = false
            }
        }
        return defaultStr
    }
    switch field.DataType {
    case schema.Bool, schema.Int, schema.Uint, schema.Float:
        sqlType = "INTEGER"
        switch {
        case field.DataType == schema.Float:
            sqlType = "FLOAT"
        case field.Size <= 8:
            sqlType = "SMALLINT"
        }
        if field.AutoIncrement {
            // sqlType += " GENERATED BY DEFAULT AS IDENTITY"
            sqlType += fmt.Sprintf(" default %s.nextval", SeqName(field.Schema.Table))
        }
    case schema.String:
        size := field.Size
        defaultSize := d.DefaultStringSize
        if size == 0 {
            if defaultSize > 0 {
                size = int(defaultSize)
            } else {
                hasIndex := field.TagSettings["INDEX"] != "" || field.TagSettings["UNIQUE"] != ""
                // TEXT, GEOMETRY or JSON column can't have a default value
                if field.PrimaryKey || field.HasDefaultValue || hasIndex {
                    size = 191 // utf8mb4
                }
            }
        }
        if size >= 8000 {
            sqlType = "CLOB"
        } else {
            sqlType = fmt.Sprintf("VARCHAR(%d)", size)
        }
        sqlType += addStringDefault()
    case schema.Time:
        sqlType = "TIMESTAMP"
        if field.NotNull || field.PrimaryKey {
            sqlType += " NOT NULL"
        }
    case schema.Bytes, JSON:
        sqlType = "VARCHAR(8000)"
    default:
        sqlType = string(field.DataType)

        if strings.EqualFold(sqlType, "text") {
            sqlType = "CLOB"
        }
        if sqlType == "" {
            panic(fmt.Sprintf("invalid sql type %s (%s) for yasdb", field.FieldType.Name(), field.FieldType.String()))
        }
        sqlType += addStringDefault()
    }
    return sqlType
}

func (d Dialector) SavePoint(tx *gorm.DB, name string) error {
    tx.Exec("SAVEPOINT " + name)
    return tx.Error
}

func (d Dialector) RollbackTo(tx *gorm.DB, name string) error {
    tx.Exec("ROLLBACK TO SAVEPOINT " + name)
    return tx.Error
}
