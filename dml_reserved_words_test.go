package yasdb

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Model with reserved word column names (all from official YashanDB reserved keywords list)
type ReservedWordModel struct {
	ID    uint   `gorm:"primaryKey;autoIncrement"`
	Order string `gorm:"column:ORDER"` // ORDER is a reserved word
	Where string `gorm:"column:WHERE"` // WHERE is a reserved word
	Date  string `gorm:"column:DATE"`  // DATE is a reserved word
	Name  string `gorm:"column:NAME"`  // NAME is not reserved
}

func openReservedWordsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "regress/regress@172.16.90.80:1688"
	if env := os.Getenv("YASDB_DSN"); env != "" {
		dsn = env
	}
	db, err := gorm.Open(Open(dsn), &gorm.Config{
		DryRun: true,
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("gorm.Open failed: %v", err)
	}
	return db
}

// TestDMLReservedWordsNotQuoted verifies that GORM's generated DML SQL properly
// quotes reserved word column names. QuoteTo (yasdb.go) wraps reserved words in
// double quotes when they appear as identifiers (column names in SET clauses,
// INSERT column lists).
//
// Note: raw SQL strings passed to .Where("date = ?") are NOT processed by QuoteTo.
// Column names in raw WHERE clauses must be quoted manually by the user.
func TestDMLReservedWordsNotQuoted(t *testing.T) {
	db := openReservedWordsTestDB(t)

	t.Run("UPDATE with reserved word columns", func(t *testing.T) {
		model := &ReservedWordModel{Date: "2024-01-01", Order: "first"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("id = ?", 1).
			Updates(model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => UPDATE with reserved word columns: %s", sql)

		// SET clause column names go through WriteQuoted -> QuoteTo
		checkUnquotedReservedWords(t, sql)
	})

	t.Run("SELECT with reserved word columns", func(t *testing.T) {
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("name = ?", "test").
			First(&ReservedWordModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => SELECT with reserved word columns: %s", sql)
	})

	t.Run("DELETE with reserved word columns", func(t *testing.T) {
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("id = ?", 1).
			Delete(&ReservedWordModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => DELETE with reserved word columns: %s", sql)
	})

	t.Run("SELECT WHERE with reserved word", func(t *testing.T) {
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("date = ?", "2024-01-01").
			Find(&[]ReservedWordModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => SELECT WHERE with reserved word: %s", sql)
	})
}

// TestCreateDMLWithReservedWords tests INSERT SQL generation with reserved word columns.
func TestCreateDMLWithReservedWords(t *testing.T) {
	db := openReservedWordsTestDB(t)

	t.Run("INSERT with reserved word columns", func(t *testing.T) {
		model := &ReservedWordModel{Date: "2024-01-01", Order: "first", Where: "here", Name: "test"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Create(&model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => INSERT with reserved word columns: %s", sql)

		checkUnquotedReservedWords(t, sql)
	})

	t.Run("INSERT single column", func(t *testing.T) {
		model := &ReservedWordModel{Name: "test"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Create(&model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => INSERT single column: %s", sql)
	})
}

// TestIsReservedWord verifies the reserved word detection function works
// against the official YashanDB reserved keywords list.
func TestIsReservedWord(t *testing.T) {
	tests := []struct {
		name string
		word string
		want bool
	}{
		// Reserved words from official YashanDB KB
		{"ORDER is reserved", "ORDER", true},
		{"WHERE is reserved", "WHERE", true},
		{"DATE is reserved", "DATE", true},
		{"SELECT is reserved", "SELECT", true},
		{"CREATE is reserved", "CREATE", true},
		{"TABLE is reserved", "TABLE", true},
		{"DROP is reserved", "DROP", true},
		{"INSERT is reserved", "INSERT", true},
		{"UPDATE is reserved", "UPDATE", true},
		{"DELETE is reserved", "DELETE", true},
		{"INDEX is reserved", "INDEX", true},
		{"ALTER is reserved", "ALTER", true},
		// Case insensitivity
		{"order lowercase is reserved", "order", true},
		{"Where mixed case is reserved", "Where", true},
		// Non-reserved words (not in V$RESERVED_WORDS)
		{"NAME is not reserved", "NAME", false},
		{"STATUS is not reserved", "STATUS", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsReservedWord(tt.word); got != tt.want {
				t.Errorf("IsReservedWord(%q) = %v, want %v", tt.word, got, tt.want)
			}
		})
	}
}

// wordsToCheck lists reserved words that should appear quoted when used as column
// names in GORM-generated SQL. Excludes words that always appear as SQL keywords
// (WHERE, ORDER, SELECT, CREATE) since they are part of valid SQL syntax.
var wordsToCheck = []string{"DATE"}

// checkUnquotedReservedWords reports if any reserved word appears in SQL
// without being surrounded by double quotes.
func checkUnquotedReservedWords(t *testing.T, sql string) {
	t.Helper()
	upperSQL := strings.ToUpper(sql)

	for _, word := range wordsToCheck {
		// Remove all "WORD" (quoted) occurrences
		withoutQuoted := strings.ReplaceAll(upperSQL, `"`+word+`"`, "")
		// Use word boundaries to avoid substring matches (e.g. "DATE" in "UPDATE")
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
		if re.MatchString(withoutQuoted) {
			t.Errorf("SQL contains unquoted reserved word '%s':\n  SQL: %s", word, sql)
		}
	}
}

func openTestDBWithCaseSensitive(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "sys/Cod-2022@172.16.90.80:1688"
	if env := os.Getenv("YASDB_DSN"); env != "" {
		dsn = env
	}
	db, err := gorm.Open(New(Config{
		DSN:                 dsn,
		NamingCaseSensitive: true,
	}), &gorm.Config{
		DryRun: true,
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("gorm.Open with NamingCaseSensitive failed: %v", err)
	}
	return db
}

// TestNamingCaseSensitive verifies that when NamingCaseSensitive is enabled,
// ALL identifiers (not just reserved words) get quoted in generated SQL.
func TestNamingCaseSensitive(t *testing.T) {
	t.Run("UPDATE quotes all columns when NamingCaseSensitive is enabled", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}
		model := &ReservedWordModel{Date: "2024-01-01", Name: "test"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("id = ?", 1).
			Updates(model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => UPDATE (case-sensitive, reserved+non-reserved): %s", sql)

		// Both reserved and non-reserved columns should be quoted
		checkColumnQuoted(t, sql, "DATE")
		checkColumnQuoted(t, sql, "NAME")
	})

	t.Run("INSERT quotes all columns when NamingCaseSensitive is enabled", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}
		model := &ReservedWordModel{Name: "test"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Create(&model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => INSERT (case-sensitive, all columns): %s", sql)

		// All columns including NAME should be quoted
		checkColumnQuoted(t, sql, "ORDER")
		checkColumnQuoted(t, sql, "WHERE")
		checkColumnQuoted(t, sql, "DATE")
		checkColumnQuoted(t, sql, "NAME")
	})

	t.Run("SELECT quotes table and columns when NamingCaseSensitive is enabled", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("name = ?", "test").
			First(&ReservedWordModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => SELECT (case-sensitive): %s", sql)
	})

	t.Run("DELETE quotes table when NamingCaseSensitive is enabled", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&ReservedWordModel{}).
			Where("id = ?", 1).
			Delete(&ReservedWordModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => DELETE (case-sensitive): %s", sql)
	})
}

// LowercaseModel uses lowercase column names to verify they get uppercased before quoting.
type LowercaseModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	UserName  string `gorm:"column:user_name"`
	CreatedAt string `gorm:"column:created_at"`
}

// TestNamingCaseSensitiveLowercase verifies that lowercase column names are
// quoted as-is when NamingCaseSensitive is enabled (no uppercase conversion).
func TestNamingCaseSensitiveLowercase(t *testing.T) {
	db := openTestDBWithCaseSensitive(t)
	db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}

	t.Run("lowercase columns quoted as-is", func(t *testing.T) {
		model := &LowercaseModel{UserName: "alice", CreatedAt: "2024-01-01"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&LowercaseModel{}).
			Where("id = ?", 1).
			Updates(model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => UPDATE lowercase columns (case-sensitive): %s", sql)

		// lowercase columns should be quoted as-is: "user_name", "created_at"
		checkColumnQuoted(t, sql, "user_name")
		checkColumnQuoted(t, sql, "created_at")
	})

	t.Run("lowercase INSERT (case-sensitive)", func(t *testing.T) {
		model := &LowercaseModel{UserName: "bob", CreatedAt: "2024-02-02"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Create(&model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => INSERT lowercase columns (case-sensitive): %s", sql)
	})

	t.Run("lowercase SELECT (case-sensitive)", func(t *testing.T) {
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&LowercaseModel{}).
			Where("user_name = ?", "alice").
			First(&LowercaseModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => SELECT lowercase columns (case-sensitive): %s", sql)
	})
}

// TestLowercaseColumnsDefault verifies lowercase columns without NamingCaseSensitive
// are NOT quoted (database auto-uppercases them).
func TestLowercaseColumnsDefault(t *testing.T) {
	db := openReservedWordsTestDB(t)

	t.Run("lowercase UPDATE (default, no case-sensitive)", func(t *testing.T) {
		model := &LowercaseModel{UserName: "alice", CreatedAt: "2024-01-01"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&LowercaseModel{}).
			Where("id = ?", 1).
			Updates(model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => UPDATE lowercase columns (default): %s", sql)
	})

	t.Run("lowercase INSERT (default, no case-sensitive)", func(t *testing.T) {
		model := &LowercaseModel{UserName: "bob", CreatedAt: "2024-02-02"}
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Create(&model)

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => INSERT lowercase columns (default): %s", sql)
	})

	t.Run("lowercase SELECT (default, no case-sensitive)", func(t *testing.T) {
		stmt := db.Session(&gorm.Session{DryRun: true}).
			Model(&LowercaseModel{}).
			Where("user_name = ?", "alice").
			First(&LowercaseModel{})

		sql := stmt.Statement.SQL.String()
		t.Logf("SQL => SELECT lowercase columns (default): %s", sql)
	})
}

// checkColumnQuoted reports if the given column appears unquoted in SQL.
func checkColumnQuoted(t *testing.T, sql string, col string) {
	t.Helper()
	upperSQL := strings.ToUpper(sql)
	withoutQuoted := strings.ReplaceAll(upperSQL, `"`+col+`"`, "")
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(col) + `\b`)
	if re.MatchString(withoutQuoted) {
		t.Errorf("column '%s' is not quoted in SQL:\n  SQL: %s", col, sql)
	}
}

// CreateTableModel uses reserved word column names for DDL testing.
type CreateTableModel struct {
	ID    uint   `gorm:"primaryKey;autoIncrement"`
	Order string `gorm:"column:ORDER"`
	Where string `gorm:"column:WHERE"`
	Date  string `gorm:"column:DATE"`
	Name  string `gorm:"column:NAME"`
}

// CreateTableLowerModel uses lowercase column names for DDL testing.
type CreateTableLowerModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	UserName  string `gorm:"column:user_name"`
	CreatedAt string `gorm:"column:created_at"`
}

// TestTryQuotifyReservedWords verifies TryQuotifyReservedWords normalizes schema
// column names without embedding SQL quote characters in DBName.
func TestTryQuotifyReservedWords(t *testing.T) {
	t.Run("quotes reserved word columns", func(t *testing.T) {
		db := openReservedWordsTestDB(t)
		stmt := &gorm.Statement{DB: db, Table: "create_table_models"}
		if err := stmt.Parse(&CreateTableModel{}); err != nil {
			t.Fatalf("stmt.Parse failed: %v", err)
		}

		migrator := db.Migrator().(Migrator)
		if err := migrator.TryQuotifyReservedWords(&CreateTableModel{}); err != nil {
			t.Fatalf("TryQuotifyReservedWords failed: %v", err)
		}

		// Re-parse to get updated schema state
		stmt2 := &gorm.Statement{DB: db, Table: "create_table_models"}
		if err := stmt2.Parse(&CreateTableModel{}); err != nil {
			t.Fatalf("stmt2.Parse failed: %v", err)
		}

		// Print schema state after quoting
		t.Logf("Schema table: %s", stmt2.Schema.Table)
		t.Logf("Schema DBNames: %v", stmt2.Schema.DBNames)

		// Generate INSERT SQL after quoting to show the effect
		model := &CreateTableModel{Date: "2024-01-01", Order: "first", Where: "here", Name: "test"}
		createStmt := db.Session(&gorm.Session{DryRun: true}).Create(model)
		sql := createStmt.Statement.SQL.String()
		t.Logf("SQL => INSERT after TryQuotifyReservedWords: %s", sql)

		// Schema DBNames should be plain uppercase names (no quote characters).
		reservedCols := []string{"ORDER", "WHERE", "DATE"}
		for _, col := range reservedCols {
			found := false
			for _, dbName := range stmt2.Schema.DBNames {
				if dbName == col {
					found = true
					if strings.Contains(dbName, `"`) {
						t.Errorf("reserved word column %q should not contain quotes in schema", dbName)
					}
					break
				}
			}
			if !found {
				t.Errorf("reserved word column %q not found in schema DBNames", col)
			}
			checkColumnQuoted(t, sql, col)
		}
		nameFound := false
		for _, dbName := range stmt2.Schema.DBNames {
			if dbName == "NAME" {
				nameFound = true
				break
			}
		}
		if !nameFound {
			t.Errorf("non-reserved column NAME should be present in schema")
		}
	})

	t.Run("quotes all columns when NamingCaseSensitive is enabled", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}

		migrator := db.Migrator().(Migrator)
		if err := migrator.TryQuotifyReservedWords(&CreateTableModel{}); err != nil {
			t.Fatalf("TryQuotifyReservedWords failed: %v", err)
		}

		stmt2 := &gorm.Statement{DB: db, Table: "create_table_models"}
		if err := stmt2.Parse(&CreateTableModel{}); err != nil {
			t.Fatalf("stmt2.Parse failed: %v", err)
		}

		t.Logf("Schema table (case-sensitive): %s", stmt2.Schema.Table)
		t.Logf("Schema DBNames (case-sensitive): %v", stmt2.Schema.DBNames)

		model := &CreateTableModel{Name: "test"}
		createStmt := db.Session(&gorm.Session{DryRun: true}).Create(model)
		sql := createStmt.Statement.SQL.String()
		t.Logf("SQL => INSERT after TryQuotifyReservedWords (case-sensitive): %s", sql)

		for _, dbName := range stmt2.Schema.DBNames {
			if strings.Contains(dbName, `"`) {
				t.Errorf("column %q should not contain quotes in schema with NamingCaseSensitive=true", dbName)
			}
		}
		if strings.Contains(stmt2.Schema.Table, `"`) {
			t.Errorf("table name %q should not contain quotes in schema with NamingCaseSensitive=true", stmt2.Schema.Table)
		}
		checkColumnQuoted(t, sql, "NAME")
	})

	t.Run("lowercase columns quoted as-is when NamingCaseSensitive", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}

		migrator := db.Migrator().(Migrator)
		if err := migrator.TryQuotifyReservedWords(&CreateTableLowerModel{}); err != nil {
			t.Fatalf("TryQuotifyReservedWords failed: %v", err)
		}

		stmt := &gorm.Statement{DB: db, Table: "create_table_lower_models"}
		if err := stmt.Parse(&CreateTableLowerModel{}); err != nil {
			t.Fatalf("stmt.Parse failed: %v", err)
		}

		t.Logf("Schema table (lowercase, case-sensitive): %s", stmt.Schema.Table)
		t.Logf("Schema DBNames (lowercase, case-sensitive): %v", stmt.Schema.DBNames)

		model := &CreateTableLowerModel{UserName: "test", CreatedAt: "2024-01-01"}
		createStmt := db.Session(&gorm.Session{DryRun: true}).Create(model)
		sql := createStmt.Statement.SQL.String()
		t.Logf("SQL => INSERT lowercase after TryQuotifyReservedWords (case-sensitive): %s", sql)

		for _, expected := range []string{"user_name", "created_at"} {
			found := false
			for _, dbName := range stmt.Schema.DBNames {
				if dbName == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected column %q not found in schema", expected)
			}
			checkColumnQuoted(t, sql, expected)
		}
	})
}

// TestMaybeQuoteName verifies the maybeQuoteName function directly.
func TestMaybeQuoteName(t *testing.T) {
	t.Run("reserved word normalized without quotes", func(t *testing.T) {
		db := openReservedWordsTestDB(t)
		migrator := db.Migrator().(Migrator)
		got := migrator.maybeQuoteName("ORDER")
		t.Logf("SQL => maybeQuoteName('ORDER') = %s", got)
		if got != "ORDER" {
			t.Errorf(`maybeQuoteName("ORDER") = %q, want ORDER`, got)
		}
	})

	t.Run("non-reserved word uppercased without quotes", func(t *testing.T) {
		db := openReservedWordsTestDB(t)
		migrator := db.Migrator().(Migrator)
		got := migrator.maybeQuoteName("name")
		t.Logf("SQL => maybeQuoteName('name') = %s", got)
		if got != "NAME" {
			t.Errorf(`maybeQuoteName("name") = %q, want NAME`, got)
		}
	})

	t.Run("case-sensitive preserves name without quotes", func(t *testing.T) {
		db := openTestDBWithCaseSensitive(t)
		db.Dialector = Dialector{Config: &Config{NamingCaseSensitive: true}}
		migrator := db.Migrator().(Migrator)
		got := migrator.maybeQuoteName("name")
		t.Logf("SQL => maybeQuoteName('name', case-sensitive) = %s", got)
		if got != "name" {
			t.Errorf(`maybeQuoteName("name") = %q, want name`, got)
		}
		got = migrator.maybeQuoteName("user_name")
		t.Logf("SQL => maybeQuoteName('user_name', case-sensitive) = %s", got)
		if got != "user_name" {
			t.Errorf(`maybeQuoteName("user_name") = %q, want user_name`, got)
		}
	})

	t.Run("already quoted name normalized", func(t *testing.T) {
		db := openReservedWordsTestDB(t)
		migrator := db.Migrator().(Migrator)
		got := migrator.maybeQuoteName(`"ORDER"`)
		t.Logf("SQL => maybeQuoteName('\"ORDER\"') = %s", got)
		if got != "ORDER" {
			t.Errorf(`maybeQuoteName("\"ORDER\"") = %q, want ORDER`, got)
		}
	})
}

// TestQuoteTo verifies the QuoteTo function directly for various scenarios.
func TestQuoteTo(t *testing.T) {
	tests := []struct {
		name      string
		dialector Dialector
		input     string
		want      string
	}{
		{
			name:      "non-reserved word, no case-sensitive",
			dialector: Dialector{Config: &Config{NamingCaseSensitive: false}},
			input:     "name",
			want:      "NAME",
		},
		{
			name:      "reserved word, no case-sensitive",
			dialector: Dialector{Config: &Config{NamingCaseSensitive: false}},
			input:     "ORDER",
			want:      `"ORDER"`,
		},
		{
			name:      "non-reserved word, case-sensitive",
			dialector: Dialector{Config: &Config{NamingCaseSensitive: true}},
			input:     "name",
			want:      `"name"`,
		},
		{
			name:      "reserved word, case-sensitive",
			dialector: Dialector{Config: &Config{NamingCaseSensitive: true}},
			input:     "ORDER",
			want:      `"ORDER"`,
		},
		{
			name:      "already quoted, no case-sensitive",
			dialector: Dialector{Config: &Config{NamingCaseSensitive: false}},
			input:     `"ORDER"`,
			want:      `"ORDER"`,
		},
		{
			name:      "already quoted lowercase, case-sensitive",
			dialector: Dialector{Config: &Config{NamingCaseSensitive: true}},
			input:     `"user_name"`,
			want:      `"user_name"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			tt.dialector.QuoteTo(&buf, tt.input)
			t.Logf("SQL => QuoteTo(%q) = %s", tt.input, buf.String())
			if got := buf.String(); got != tt.want {
				t.Errorf("QuoteTo(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
