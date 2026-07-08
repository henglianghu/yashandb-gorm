package yasdb

import (
	"bytes"
	"database/sql"
	"fmt"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testLogBuf struct{ b bytes.Buffer }

func (l *testLogBuf) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&l.b, format, args...)
}

// TestJoin_Diagnostic 区分「库不支持 JOIN」与「驱动扫描映射问题」。
func TestJoin_Diagnostic(t *testing.T) {
	var buf testLogBuf
	db, err := gorm.Open(Open(testDSN), &gorm.Config{
		Logger: logger.New(&buf, logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	setupJoinTablesDefault(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_join_book CASCADE CONSTRAINTS").Error
		_ = db.Exec("DROP TABLE ut_join_author CASCADE CONSTRAINTS").Error
	})

	_ = db.Create(&joinAuthor{ID: 99, Name: "DiagAlice"})
	_ = db.Create(&joinBook{ID: 99, Title: "DiagBook", AuthorID: 99})

	// 1) 原生 SQL JOIN
	var title, authorName string
	err = db.Raw(`
		SELECT b.title, a.name
		FROM ut_join_book b
		LEFT JOIN ut_join_author a ON b.author_id = a.id
		WHERE b.id = ?
	`, 99).Row().Scan(&title, &authorName)
	if err != nil {
		t.Fatalf("raw JOIN failed (若此处失败才可能是库不支持 JOIN): %v", err)
	}
	t.Logf("raw JOIN ok: title=%q authorName=%q", title, authorName)
	if authorName != "DiagAlice" {
		t.Fatalf("raw JOIN returned wrong author: %q", authorName)
	}

	// 2) GORM Joins —— SQL 能执行、主表字段能扫到
	buf.b.Reset()
	var books []joinBook
	if err := db.Joins("Author").Where("ut_join_book.id = ?", 99).Find(&books).Error; err != nil {
		t.Fatalf("GORM Joins SQL error: %v\n%s", err, buf.b.String())
	}
	t.Logf("GORM Joins SQL:\n%s", buf.b.String())
	t.Logf("GORM result: len=%d book.Title=%q Author=%v", len(books), books[0].Title, books[0].Author)
	if books[0].Title != "DiagBook" {
		t.Fatalf("main table scan failed")
	}

	// 3) 底层驱动返回的列名（与 GORM 实际生成的 SELECT 一致）
	sqlDB, _ := db.DB()
	rows, err := sqlDB.Query(`
		SELECT ut_join_book.ID, ut_join_book.TITLE, ut_join_book.AUTHOR_ID,
		       Author.ID AS Author__ID, Author.NAME AS Author__NAME
		FROM ut_join_book
		LEFT JOIN ut_join_author Author ON ut_join_book.AUTHOR_ID = Author.ID
		WHERE ut_join_book.ID = 99
	`)
	if err != nil {
		t.Fatalf("GORM-style JOIN query failed: %v", err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	t.Logf("driver column names: %v", cols)

	if rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		_ = rows.Scan(ptrs...)
		for i, c := range cols {
			t.Logf("  col[%s] = %v (valid=%v)", c, vals[i].String, vals[i].Valid)
		}
	}
}
