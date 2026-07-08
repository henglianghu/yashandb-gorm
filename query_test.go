package yasdb

import (
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMismatchedCaseHandler_DefaultMode(t *testing.T) {
	idField := &schema.Field{Name: "ID", DBName: "ID"}
	nameField := &schema.Field{Name: "Name", DBName: "USER_NAME"}
	s := &schema.Schema{
		Fields:         []*schema.Field{idField, nameField},
		DBNames:        []string{"ID", "USER_NAME"},
		FieldsByDBName: map[string]*schema.Field{"ID": idField, "USER_NAME": nameField},
	}

	stmt := &gorm.Statement{Schema: s}
	MismatchedCaseHandler(&gorm.DB{Config: &gorm.Config{}, Statement: stmt})

	if stmt.ColumnMapping == nil {
		t.Fatal("ColumnMapping should be set in default mode")
	}
	if stmt.ColumnMapping["ID"] != "ID" {
		t.Fatalf("expected ID->ID, got %q", stmt.ColumnMapping["ID"])
	}
	if stmt.ColumnMapping["USER_NAME"] != "Name" {
		t.Fatalf("expected USER_NAME->Name, got %q", stmt.ColumnMapping["USER_NAME"])
	}
}

func TestMismatchedCaseHandler_CaseSensitiveModeSkipped(t *testing.T) {
	idField := &schema.Field{Name: "ID", DBName: "id"}
	nameField := &schema.Field{Name: "Name", DBName: "user_name"}
	s := &schema.Schema{
		Fields:         []*schema.Field{idField, nameField},
		DBNames:        []string{"id", "user_name"},
		FieldsByDBName: map[string]*schema.Field{"id": idField, "user_name": nameField},
	}

	stmt := &gorm.Statement{Schema: s}
	db := &gorm.DB{
		Config: &gorm.Config{
			Dialector: &Dialector{Config: &Config{NamingCaseSensitive: true}},
		},
		Statement: stmt,
	}
	MismatchedCaseHandler(db)

	if stmt.ColumnMapping != nil {
		t.Fatalf("ColumnMapping should be nil in NamingCaseSensitive mode, got %v", stmt.ColumnMapping)
	}
}
