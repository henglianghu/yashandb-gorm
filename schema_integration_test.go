package yasdb

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// indexRowModel 显式 column:task_index，规范化后应为 TASK_INDEX
type indexRowModel struct {
	ID        int    `gorm:"primaryKey"`
	TaskIndex string `gorm:"column:task_index"`
}

func (indexRowModel) TableName() string { return "ut_schema_index" }

type mixedColumnModel struct {
	ID       int    `gorm:"primaryKey"`
	Name     string
	NoteText string `gorm:"column:note_text"`
	Payload  []byte `gorm:"column:raw_payload"`
}

func (mixedColumnModel) TableName() string { return "ut_schema_mixed" }

// indexColModel 保留字列 index，用于 Migrator 引号逻辑测试
type indexColModel struct {
	ID    int    `gorm:"primaryKey"`
	Value string `gorm:"column:index"`
}

func (indexColModel) TableName() string { return "ut_schema_index_col" }

func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestSchemaNormalize_CallbackAutoApplyOnCreate(t *testing.T) {
	db := openIntegrationDB(t)
	setupDocumentBlobTable(t, db)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error })

	tx := db.Create(&documentTestModel{ID: 1, Title: "t", Content: "c", RawData: []byte("x")})
	if tx.Error != nil {
		t.Fatalf("create: %v", tx.Error)
	}

	field := tx.Statement.Schema.LookUpField("RAW_DATA")
	if field == nil {
		t.Fatal("callback should normalize schema so LookUpField(RAW_DATA) works")
	}

	var found documentTestModel
	if err := db.First(&found, 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(found.RawData) != "x" {
		t.Fatalf("expected raw_data=x, got %q", found.RawData)
	}
}

func TestSchemaNormalize_CreateSQLUsesUppercaseExplicitColumn(t *testing.T) {
	db := openIntegrationDB(t)
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Create(&documentTestModel{ID: 1, Title: "t", Content: "c", RawData: []byte("b")})
	})
	if strings.Contains(sql, "raw_data") {
		t.Fatalf("normalized INSERT should not use lowercase raw_data: %s", sql)
	}
	if !strings.Contains(sql, "RAW_DATA") {
		t.Fatalf("normalized INSERT should use RAW_DATA: %s", sql)
	}
}

func TestSchemaNormalize_DocumentBLOB_UpdateAndFind(t *testing.T) {
	db := openIntegrationDB(t)
	setupDocumentBlobTable(t, db)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error })

	doc := documentTestModel{ID: 1, Title: "t", Content: "c", RawData: []byte("v1")}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.Model(&documentTestModel{}).Where("id = ?", 1).
		Updates(map[string]interface{}{"raw_data": []byte("v2")}).Error; err != nil {
		t.Fatalf("update by map key: %v", err)
	}

	var found documentTestModel
	if err := db.First(&found, 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(found.RawData) != "v2" {
		t.Fatalf("expected v2 after update, got %q", found.RawData)
	}
}

func TestSchemaNormalize_DocumentBLOB_LargePayload(t *testing.T) {
	db := openIntegrationDB(t)
	setupDocumentBlobTable(t, db)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error })

	large := make([]byte, 4096)
	for i := range large {
		large[i] = byte(i % 256)
	}

	if err := db.Create(&documentTestModel{ID: 1, Title: "big", Content: "c", RawData: large}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var found documentTestModel
	if err := db.First(&found, 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(found.RawData) != len(large) {
		t.Fatalf("expected blob len %d, got %d", len(large), len(found.RawData))
	}
	for i := range large {
		if found.RawData[i] != large[i] {
			t.Fatalf("blob content mismatch at %d", i)
		}
	}
}

func TestSchemaNormalize_MixedExplicitAndDefaultColumns_CRUD(t *testing.T) {
	db := openIntegrationDB(t)
	_ = db.Exec("DROP TABLE ut_schema_mixed CASCADE CONSTRAINTS").Error
	if err := db.Exec(`CREATE TABLE ut_schema_mixed (
		id INT PRIMARY KEY,
		name VARCHAR(64),
		note_text VARCHAR(128),
		raw_payload BLOB
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_schema_mixed CASCADE CONSTRAINTS").Error })

	row := mixedColumnModel{ID: 1, Name: "n1", NoteText: "note", Payload: []byte("p1")}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var found mixedColumnModel
	if err := db.Where("note_text = ?", "note").First(&found).Error; err != nil {
		t.Fatalf("query by explicit column: %v", err)
	}
	if found.Name != "n1" || string(found.Payload) != "p1" {
		t.Fatalf("unexpected row: %+v", found)
	}

	if err := db.Model(&found).Update("Name", "n2").Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := db.Delete(&mixedColumnModel{}, 1).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestSchemaNormalize_ExplicitColumnTag_CRUD(t *testing.T) {
	db := openIntegrationDB(t)
	_ = db.Exec("DROP TABLE ut_schema_index CASCADE CONSTRAINTS").Error
	if err := db.Exec(`CREATE TABLE ut_schema_index (id INT PRIMARY KEY, task_index VARCHAR(64))`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_schema_index CASCADE CONSTRAINTS").Error })

	row := indexRowModel{ID: 1, TaskIndex: "idx-1"}
	tx := db.Create(&row)
	if tx.Error != nil {
		t.Fatalf("create: %v", tx.Error)
	}

	if tx.Statement.Schema.LookUpField("TASK_INDEX") == nil {
		t.Fatal("TASK_INDEX should be lookup-able after normalization")
	}

	var found indexRowModel
	if err := db.Where("task_index = ?", "idx-1").First(&found).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if found.TaskIndex != "idx-1" {
		t.Fatalf("unexpected TaskIndex: %q", found.TaskIndex)
	}
}

func TestSchemaNormalize_MigratorQuotifyReservedWordsCompatible(t *testing.T) {
	db := openIntegrationDB(t)
	m := db.Dialector.Migrator(db).(Migrator)

	if err := db.Statement.Parse(&indexColModel{}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	NormalizeSchemaColumnNames(db.Statement.Schema)

	if err := m.TryQuotifyReservedWords(&indexColModel{}); err != nil {
		t.Fatalf("TryQuotifyReservedWords should not error on normalized INDEX column: %v", err)
	}
}

func TestSchemaNormalize_QueryCallbackApplies(t *testing.T) {
	db := openIntegrationDB(t)
	setupDocumentBlobTable(t, db)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error })

	_ = db.Create(&documentTestModel{ID: 1, Title: "t", Content: "c", RawData: []byte("q")})

	var found documentTestModel
	if err := db.Where("id = ?", 1).Find(&found).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if string(found.RawData) != "q" {
		t.Fatalf("query callback should normalize and read blob, got %q", found.RawData)
	}
}

func TestSchemaNormalize_DeleteCallbackApplies(t *testing.T) {
	db := openIntegrationDB(t)
	setupDocumentBlobTable(t, db)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE ut_document_blob CASCADE CONSTRAINTS").Error })

	_ = db.Create(&documentTestModel{ID: 1, Title: "t", Content: "c", RawData: []byte("d")})
	if err := db.Delete(&documentTestModel{}, 1).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	if err := db.Model(&documentTestModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after delete, got %d", count)
	}
}
