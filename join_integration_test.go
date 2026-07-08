package yasdb

import (
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- 默认模式：标准字段名（无显式 column 标签）---

type joinAuthor struct {
	ID   int `gorm:"primaryKey"`
	Name string
}

func (joinAuthor) TableName() string { return "ut_join_author" }

type joinBook struct {
	ID       int `gorm:"primaryKey"`
	Title    string
	AuthorID int
	Author   *joinAuthor `gorm:"foreignKey:AuthorID"`
}

func (joinBook) TableName() string { return "ut_join_book" }

// --- 默认模式：关联表使用显式下划线列名 ---

type joinAuthorSnake struct {
	ID         int    `gorm:"primaryKey"`
	AuthorName string `gorm:"column:author_name"`
}

func (joinAuthorSnake) TableName() string { return "ut_join_author_snake" }

type joinBookSnake struct {
	ID       int `gorm:"primaryKey"`
	Title    string
	AuthorID int
	Author   *joinAuthorSnake `gorm:"foreignKey:AuthorID"`
}

func (joinBookSnake) TableName() string { return "ut_join_book_snake" }

// --- 敏感模式：表/列使用引号保留小写 ---

type joinAuthorCS struct {
	ID   int    `gorm:"primaryKey;column:id"`
	Name string `gorm:"column:name"`
}

func (joinAuthorCS) TableName() string { return "ut_join_author_cs" }

type joinBookCS struct {
	ID       int    `gorm:"primaryKey;column:id"`
	Title    string `gorm:"column:title"`
	AuthorID int    `gorm:"column:author_id"`
	Author   *joinAuthorCS `gorm:"foreignKey:AuthorID;references:ID"`
}

func (joinBookCS) TableName() string { return "ut_join_book_cs" }

func setupJoinTablesDefault(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec("DROP TABLE ut_join_book CASCADE CONSTRAINTS").Error
	_ = db.Exec("DROP TABLE ut_join_author CASCADE CONSTRAINTS").Error
	if err := db.Exec(`CREATE TABLE ut_join_author (
		id INT PRIMARY KEY,
		name VARCHAR(64)
	)`).Error; err != nil {
		t.Fatalf("create ut_join_author: %v", err)
	}
	if err := db.Exec(`CREATE TABLE ut_join_book (
		id INT PRIMARY KEY,
		title VARCHAR(128),
		author_id INT
	)`).Error; err != nil {
		t.Fatalf("create ut_join_book: %v", err)
	}
}

func setupJoinTablesSnake(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec("DROP TABLE ut_join_book_snake CASCADE CONSTRAINTS").Error
	_ = db.Exec("DROP TABLE ut_join_author_snake CASCADE CONSTRAINTS").Error
	if err := db.Exec(`CREATE TABLE ut_join_author_snake (
		id INT PRIMARY KEY,
		author_name VARCHAR(64)
	)`).Error; err != nil {
		t.Fatalf("create ut_join_author_snake: %v", err)
	}
	if err := db.Exec(`CREATE TABLE ut_join_book_snake (
		id INT PRIMARY KEY,
		title VARCHAR(128),
		author_id INT
	)`).Error; err != nil {
		t.Fatalf("create ut_join_book_snake: %v", err)
	}
}

func setupJoinTablesCaseSensitive(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec(`DROP TABLE "ut_join_book_cs" CASCADE CONSTRAINTS`).Error
	_ = db.Exec(`DROP TABLE "ut_join_author_cs" CASCADE CONSTRAINTS`).Error
	if err := db.Exec(`CREATE TABLE "ut_join_author_cs" (
		"id" INT PRIMARY KEY,
		"name" VARCHAR(64)
	)`).Error; err != nil {
		t.Fatalf("create ut_join_author_cs: %v", err)
	}
	if err := db.Exec(`CREATE TABLE "ut_join_book_cs" (
		"id" INT PRIMARY KEY,
		"title" VARCHAR(128),
		"author_id" INT
	)`).Error; err != nil {
		t.Fatalf("create ut_join_book_cs: %v", err)
	}
}

func openJoinTestDB(t *testing.T, caseSensitive bool) *gorm.DB {
	t.Helper()
	var db *gorm.DB
	var err error
	if caseSensitive {
		db, err = gorm.Open(New(Config{
			DSN:                 testDSN,
			NamingCaseSensitive: true,
		}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	} else {
		db, err = gorm.Open(Open(testDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	}
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

// TestJoin_DefaultMode_SimpleJoin 默认模式下简单 Join，关联对象应能回填。
func TestJoin_DefaultMode_SimpleJoin(t *testing.T) {
	db := openJoinTestDB(t, false)
	setupJoinTablesDefault(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_join_book CASCADE CONSTRAINTS").Error
		_ = db.Exec("DROP TABLE ut_join_author CASCADE CONSTRAINTS").Error
	})

	if err := db.Create(&joinAuthor{ID: 1, Name: "Alice"}).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := db.Create(&joinBook{ID: 1, Title: "Go Guide", AuthorID: 1}).Error; err != nil {
		t.Fatalf("create book: %v", err)
	}

	var books []joinBook
	if err := db.Joins("Author").Find(&books).Error; err != nil {
		t.Fatalf("join find: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].Author == nil {
		t.Fatal("Author should not be nil after Joins in default mode")
	}
	if books[0].Author.Name != "Alice" {
		t.Fatalf("expected Author.Name=Alice, got %q", books[0].Author.Name)
	}
}

// TestJoin_DefaultMode_Preload 默认模式下 Preload，关联对象应能回填。
func TestJoin_DefaultMode_Preload(t *testing.T) {
	db := openJoinTestDB(t, false)
	setupJoinTablesDefault(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_join_book CASCADE CONSTRAINTS").Error
		_ = db.Exec("DROP TABLE ut_join_author CASCADE CONSTRAINTS").Error
	})

	if err := db.Create(&joinAuthor{ID: 2, Name: "Bob"}).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := db.Create(&joinBook{ID: 2, Title: "DB Guide", AuthorID: 2}).Error; err != nil {
		t.Fatalf("create book: %v", err)
	}

	var books []joinBook
	if err := db.Preload("Author").Find(&books, 2).Error; err != nil {
		t.Fatalf("preload find: %v", err)
	}
	if len(books) != 1 || books[0].Author == nil {
		t.Fatalf("preload should populate Author, got %+v", books)
	}
	if books[0].Author.Name != "Bob" {
		t.Fatalf("expected Author.Name=Bob, got %q", books[0].Author.Name)
	}
}

// TestJoin_DefaultMode_ExplicitSnakeColumn 关联表含 gorm:"column:author_name" 时 Join 是否仍能回填。
func TestJoin_DefaultMode_ExplicitSnakeColumn(t *testing.T) {
	db := openJoinTestDB(t, false)
	setupJoinTablesSnake(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_join_book_snake CASCADE CONSTRAINTS").Error
		_ = db.Exec("DROP TABLE ut_join_author_snake CASCADE CONSTRAINTS").Error
	})

	if err := db.Create(&joinAuthorSnake{ID: 1, AuthorName: "Carol"}).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := db.Create(&joinBookSnake{ID: 1, Title: "Snake Join", AuthorID: 1}).Error; err != nil {
		t.Fatalf("create book: %v", err)
	}

	var books []joinBookSnake
	if err := db.Joins("Author").Find(&books).Error; err != nil {
		t.Fatalf("join find: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].Author == nil {
		t.Fatal("Author should not be nil when joined table uses column:author_name")
	}
	if books[0].Author.AuthorName != "Carol" {
		t.Fatalf("expected AuthorName=Carol, got %q", books[0].Author.AuthorName)
	}
}

// TestJoin_NamingCaseSensitive_SimpleJoin 敏感模式下简单 Join。
func TestJoin_NamingCaseSensitive_SimpleJoin(t *testing.T) {
	db := openJoinTestDB(t, true)
	setupJoinTablesCaseSensitive(t, db)
	t.Cleanup(func() {
		_ = db.Exec(`DROP TABLE "ut_join_book_cs" CASCADE CONSTRAINTS`).Error
		_ = db.Exec(`DROP TABLE "ut_join_author_cs" CASCADE CONSTRAINTS`).Error
	})

	if err := db.Create(&joinAuthorCS{ID: 1, Name: "Dave"}).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := db.Create(&joinBookCS{ID: 1, Title: "CS Join", AuthorID: 1}).Error; err != nil {
		t.Fatalf("create book: %v", err)
	}

	var books []joinBookCS
	if err := db.Joins("Author").Find(&books).Error; err != nil {
		t.Fatalf("join find: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].Author == nil {
		t.Fatal("Author should not be nil in NamingCaseSensitive join mode")
	}
	if books[0].Author.Name != "Dave" {
		t.Fatalf("expected Author.Name=Dave, got %q", books[0].Author.Name)
	}
}
