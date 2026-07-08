package yasdb

import (
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type documentTestModel struct {
	ID      int `gorm:"primaryKey"`
	Title   string
	Content string
	RawData []byte `gorm:"column:raw_data"`
}

func (documentTestModel) TableName() string {
	return "ut_document_blob"
}

func setupDocumentBlobTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error
	if err := db.Exec(`CREATE TABLE ut_document_blob (
		id INT PRIMARY KEY,
		title VARCHAR(128),
		content CLOB,
		raw_data BLOB
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
}

func setupDocumentBlobTableCaseSensitive(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec(`DROP TABLE "ut_document_blob" CASCADE CONSTRAINTS`).Error
	if err := db.Exec(`CREATE TABLE "ut_document_blob" (
		"id" INT PRIMARY KEY,
		"title" VARCHAR(128),
		"content" CLOB,
		"raw_data" BLOB
	)`).Error; err != nil {
		t.Fatalf("create case-sensitive table: %v", err)
	}
}

func TestDocumentBLOB_ReadWithExplicitColumnTag(t *testing.T) {
	db, err := gorm.Open(Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	setupDocumentBlobTable(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error
	})

	doc := documentTestModel{ID: 1, Title: "t", Content: "c", RawData: []byte("b")}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var found documentTestModel
	if err := db.First(&found, 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if found.Title != "t" || found.Content != "c" {
		t.Fatalf("unexpected varchar/clob fields: %+v", found)
	}
	if len(found.RawData) == 0 {
		t.Fatalf("BLOB should be readable via First, got empty RawData")
	}
	if string(found.RawData) != "b" {
		t.Fatalf("unexpected raw_data: %q", found.RawData)
	}
}

func TestDocumentBLOB_SchemaColumnNameNormalized(t *testing.T) {
	db, err := gorm.Open(Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.Statement.Parse(&documentTestModel{}); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	NormalizeSchemaColumnNames(db.Statement.Schema)

	field := db.Statement.Schema.LookUpField("RAW_DATA")
	if field == nil {
		t.Fatal("schema should map RAW_DATA after normalization")
	}
	if field.DBName != "RAW_DATA" {
		t.Fatalf("expected DBName RAW_DATA, got %q", field.DBName)
	}
}

func TestDocumentBLOB_NamingCaseSensitive_RoundTrip(t *testing.T) {
	db, err := gorm.Open(New(Config{
		DSN:                 testDSN,
		NamingCaseSensitive: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	setupDocumentBlobTableCaseSensitive(t, db)
	t.Cleanup(func() {
		_ = db.Exec(`DROP TABLE "ut_document_blob" CASCADE CONSTRAINTS`).Error
	})

	doc := documentTestModel{ID: 2, Title: "t2", Content: "c2", RawData: []byte("blob-sensitive")}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var found documentTestModel
	if err := db.First(&found, 2).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if found.Title != "t2" || found.Content != "c2" {
		t.Fatalf("unexpected varchar/clob fields: %+v", found)
	}
	if string(found.RawData) != "blob-sensitive" {
		t.Fatalf("unexpected raw_data: %q", found.RawData)
	}
}
