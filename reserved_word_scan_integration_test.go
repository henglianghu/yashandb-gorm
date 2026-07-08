package yasdb

import (
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// metaDatabase mirrors colleague main.go: User/Password map to reserved words USER/PASSWORD.
type metaDatabase struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;size:128"`
	Host      string    `gorm:"size:255"`
	Port      int
	User      string `gorm:"size:128"`
	Password  string `gorm:"size:255"`
	Status    string `gorm:"size:32;default:inactive"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (metaDatabase) TableName() string { return "ut_meta_database" }

func TestReservedWordColumns_UserPassword_ReadAfterAutoMigrate(t *testing.T) {
	db, err := gorm.Open(Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	_ = db.Exec(`DROP TABLE ut_meta_database CASCADE CONSTRAINTS`).Error
	t.Cleanup(func() {
		_ = db.Exec(`DROP TABLE ut_meta_database CASCADE CONSTRAINTS`).Error
	})

	if err := db.AutoMigrate(&metaDatabase{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	row := &metaDatabase{
		Name:     "scan_test",
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "admin",
		Password: "secret123",
		Status:   "active",
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var found metaDatabase
	if err := db.First(&found, row.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if found.User != "admin" {
		t.Fatalf("expected User=admin, got %q", found.User)
	}
	if found.Password != "secret123" {
		t.Fatalf("expected Password=secret123, got %q", found.Password)
	}
}
