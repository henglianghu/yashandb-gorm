package yasdb

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const testDSN = "MY_TEST003/123456@172.16.90.67:23500"

type conflictTestUser struct {
	ID    int `gorm:"primaryKey"`
	Name  string
	Email string
}

func (conflictTestUser) TableName() string {
	return "ut_conflict_user"
}

type conflictTestUserWithAge struct {
	ID   int `gorm:"primaryKey"`
	Name string
	Age  int
}

func (conflictTestUserWithAge) TableName() string {
	return "ut_conflict_user_age"
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func onConflictSQL(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB) *gorm.DB) string {
	t.Helper()
	return db.ToSQL(fn)
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func setupConflictUserTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	_ = db.Exec("DROP TABLE ut_conflict_user CASCADE CONSTRAINTS").Error
	if err := db.Exec(`CREATE TABLE ut_conflict_user (
		id INT PRIMARY KEY,
		name VARCHAR(30),
		email VARCHAR(128)
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	_ = db.Exec("CREATE UNIQUE INDEX idx_ut_conflict_user_email ON ut_conflict_user (email)").Error
}

func TestResolveOnConflictDoUpdates_DoNothing(t *testing.T) {
	db := openTestDB(t)
	if err := db.Statement.Parse(&conflictTestUser{}); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	got := resolveOnConflictDoUpdates(clause.OnConflict{DoNothing: true}, db.Statement)
	if len(got) != 0 {
		t.Fatalf("expected empty updates, got %d", len(got))
	}
}

func TestResolveOnConflictDoUpdates_ExplicitColumns(t *testing.T) {
	db := openTestDB(t)
	if err := db.Statement.Parse(&conflictTestUser{}); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	explicit := clause.AssignmentColumns([]string{"name"})
	got := resolveOnConflictDoUpdates(clause.OnConflict{DoUpdates: explicit}, db.Statement)
	if len(got) != 1 || got[0].Column.Name != "name" {
		t.Fatalf("unexpected updates: %+v", got)
	}
}

func TestResolveOnConflictDoUpdates_DefaultNonPrimaryKey(t *testing.T) {
	db := openTestDB(t)
	if err := db.Statement.Parse(&conflictTestUser{}); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	got := resolveOnConflictDoUpdates(clause.OnConflict{}, db.Statement)
	if len(got) != 2 {
		t.Fatalf("expected 2 non-primary columns, got %d: %+v", len(got), got)
	}
	names := map[string]bool{got[0].Column.Name: true, got[1].Column.Name: true}
	if !names["NAME"] || !names["EMAIL"] {
		t.Fatalf("expected NAME and EMAIL, got %+v", got)
	}
}

func TestOnConflictSQL_DoNothing(t *testing.T) {
	db := openTestDB(t)
	sql := onConflictSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&conflictTestUser{
			ID: 1, Name: "orig", Email: "test@example.com",
		})
	})
	if !strings.Contains(strings.ToUpper(sql), "ON DUPLICATE KEY UPDATE ID=ID") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestOnConflictSQL_AssignmentColumns(t *testing.T) {
	db := openTestDB(t)
	sql := onConflictSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"name"}),
		}).Create(&conflictTestUser{ID: 1, Name: "updated", Email: "test@example.com"})
	})
	if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE NAME=VALUES(NAME)") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestOnConflictSQL_WithTable(t *testing.T) {
	db := openTestDB(t)
	sql := onConflictSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return tx.Table("ut_conflict_user").Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"name"}),
		}).Create(&conflictTestUser{ID: 1, Name: "updated", Email: "test@example.com"})
	})
	if strings.Contains(sql, "ut_conflict_user.name=") || strings.Contains(sql, "UT_CONFLICT_USER.NAME=") {
		t.Fatalf("assignment column should not contain table prefix: %s", sql)
	}
	if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE NAME=VALUES(NAME)") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestOnConflictSQL_Expr(t *testing.T) {
	db := openTestDB(t)
	sql := onConflictSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clause.OnConflict{
			DoUpdates: []clause.Assignment{
				{Column: clause.Column{Name: "age"}, Value: clause.Expr{SQL: "age + 1"}},
			},
		}).Create(&conflictTestUserWithAge{ID: 1, Name: "a", Age: 10})
	})
	if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE AGE=age + 1") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestOnConflictSQL_DefaultEmptyDoUpdates(t *testing.T) {
	db := openTestDB(t)
	sql := onConflictSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clause.OnConflict{}).Create(&conflictTestUser{
			ID: 1, Name: "updated", Email: "new@example.com",
		})
	})
	if !containsAll(sql, "NAME=VALUES(NAME)", "EMAIL=VALUES(EMAIL)") {
		t.Fatalf("expected default non-primary key updates, got: %s", sql)
	}
}

func TestOnConflictSQL_UpdateAll(t *testing.T) {
	db := openTestDB(t)
	sql := onConflictSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&conflictTestUser{
			ID: 1, Name: "updated", Email: "new@example.com",
		})
	})
	if !containsAll(sql, "NAME=VALUES(NAME)", "EMAIL=VALUES(EMAIL)") {
		t.Fatalf("expected UpdateAll non-primary key updates, got: %s", sql)
	}
}

func TestOnConflictIntegration_DoNothingPreservesData(t *testing.T) {
	db := openTestDB(t)
	setupConflictUserTable(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_conflict_user CASCADE CONSTRAINTS").Error
	})

	user := conflictTestUser{ID: 1, Name: "orig", Email: "test@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&conflictTestUser{
		ID: 2, Name: "should-not-apply", Email: "test@example.com",
	}).Error
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var found conflictTestUser
	if err := db.Where("email = ?", "test@example.com").First(&found).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if found.Name != "orig" || found.ID != 1 {
		t.Fatalf("DoNothing should preserve row, got ID=%d Name=%q", found.ID, found.Name)
	}
}

func TestOnConflictIntegration_AssignmentColumns(t *testing.T) {
	db := openTestDB(t)
	setupConflictUserTable(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_conflict_user CASCADE CONSTRAINTS").Error
	})

	user := conflictTestUser{ID: 1, Name: "orig", Email: "test@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"name"}),
	}).Create(&conflictTestUser{ID: 99, Name: "updated", Email: "test@example.com"}).Error
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var found conflictTestUser
	if err := db.Where("email = ?", "test@example.com").First(&found).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if found.Name != "updated" {
		t.Fatalf("expected name=updated, got %q", found.Name)
	}
	if found.ID != 1 {
		t.Fatalf("primary key should remain 1 on email conflict, got %d", found.ID)
	}
}

func TestOnConflictIntegration_DefaultUpdatesNonPrimaryKey(t *testing.T) {
	db := openTestDB(t)
	setupConflictUserTable(t, db)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE ut_conflict_user CASCADE CONSTRAINTS").Error
	})

	user := conflictTestUser{ID: 1, Name: "orig", Email: "orig@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
	}).Create(&conflictTestUser{ID: 1, Name: "updated", Email: "new@example.com"}).Error
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var found conflictTestUser
	if err := db.First(&found, 1).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if found.Name != "updated" || found.Email != "new@example.com" {
		t.Fatalf("expected both non-primary columns updated, got Name=%q Email=%q", found.Name, found.Email)
	}
}
