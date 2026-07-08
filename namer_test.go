package yasdb

import (
	"testing"

	"gorm.io/gorm/schema"
)

func TestTryRemoveQuotes(t *testing.T) {
	name := []string{`"USER"`, `USER`, `"U"`, `"U`, `U`}
	wantName := []string{`USER`, `USER`, `U`, `"U`, `U`}

	for i, v := range name {
		getName := TryRemoveQuotes(v)
		if getName != wantName[i] {
			t.Fatalf("TryRemoveQuotes function err: name - %s; getName - %s; wantName - %s", v, getName, wantName[i])
		}
	}
}

func TestNormalizeSchemaColumnNames_ExplicitColumnTag(t *testing.T) {
	rawField := &schema.Field{Name: "RawData", DBName: "raw_data"}
	titleField := &schema.Field{Name: "Title", DBName: "TITLE"}
	s := &schema.Schema{
		Fields:         []*schema.Field{titleField, rawField},
		DBNames:        []string{"TITLE", "raw_data"},
		FieldsByDBName: map[string]*schema.Field{"TITLE": titleField, "raw_data": rawField},
	}

	NormalizeSchemaColumnNames(s)

	if rawField.DBName != "RAW_DATA" {
		t.Fatalf("expected RAW_DATA, got %q", rawField.DBName)
	}
	if s.FieldsByDBName["RAW_DATA"] != rawField {
		t.Fatalf("FieldsByDBName should contain RAW_DATA key")
	}
	if _, ok := s.FieldsByDBName["raw_data"]; ok {
		t.Fatalf("lowercase raw_data key should be removed")
	}
	if s.DBNames[1] != "RAW_DATA" {
		t.Fatalf("expected DBNames[1]=RAW_DATA, got %q", s.DBNames[1])
	}
}

func TestNormalizeSchemaColumnNames_Idempotent(t *testing.T) {
	field := &schema.Field{Name: "RawData", DBName: "RAW_DATA"}
	s := &schema.Schema{
		Fields:         []*schema.Field{field},
		DBNames:        []string{"RAW_DATA"},
		FieldsByDBName: map[string]*schema.Field{"RAW_DATA": field},
	}

	NormalizeSchemaColumnNames(s)
	NormalizeSchemaColumnNames(s)

	if field.DBName != "RAW_DATA" {
		t.Fatalf("normalize should be idempotent, got %q", field.DBName)
	}
}

func TestNormalizeSchemaColumnNames_PrimaryFieldDBNames(t *testing.T) {
	idField := &schema.Field{Name: "ID", DBName: "id", PrimaryKey: true}
	nameField := &schema.Field{Name: "Name", DBName: "name"}
	s := &schema.Schema{
		Fields:              []*schema.Field{idField, nameField},
		PrimaryFields:       []*schema.Field{idField},
		DBNames:             []string{"id", "name"},
		PrimaryFieldDBNames: []string{"id"},
		FieldsByDBName:      map[string]*schema.Field{"id": idField, "name": nameField},
	}

	NormalizeSchemaColumnNames(s)

	if idField.DBName != "ID" || nameField.DBName != "NAME" {
		t.Fatalf("unexpected DBName: id=%q name=%q", idField.DBName, nameField.DBName)
	}
	if len(s.PrimaryFieldDBNames) != 1 || s.PrimaryFieldDBNames[0] != "ID" {
		t.Fatalf("unexpected PrimaryFieldDBNames: %v", s.PrimaryFieldDBNames)
	}
}

func TestNormalizeSchemaColumnNames_ReservedWordColumnTag(t *testing.T) {
	field := &schema.Field{Name: "Value", DBName: "index"}
	s := &schema.Schema{
		Fields:         []*schema.Field{field},
		DBNames:        []string{"index"},
		FieldsByDBName: map[string]*schema.Field{"index": field},
	}

	NormalizeSchemaColumnNames(s)

	if field.DBName != "INDEX" {
		t.Fatalf("expected INDEX, got %q", field.DBName)
	}
	if s.FieldsByDBName["INDEX"] != field {
		t.Fatal("FieldsByDBName should contain INDEX")
	}
	if !IsReservedWord(field.DBName) {
		t.Fatal("INDEX should remain detectable as reserved word after normalization")
	}
}

func TestNormalizeSchemaColumnNames_MixedExplicitAndDefault(t *testing.T) {
	title := &schema.Field{Name: "Title", DBName: "TITLE"}
	raw := &schema.Field{Name: "RawData", DBName: "raw_data"}
	s := &schema.Schema{
		Fields:         []*schema.Field{title, raw},
		DBNames:        []string{"TITLE", "raw_data"},
		FieldsByDBName: map[string]*schema.Field{"TITLE": title, "raw_data": raw},
	}

	NormalizeSchemaColumnNames(s)

	if s.DBNames[0] != "TITLE" || s.DBNames[1] != "RAW_DATA" {
		t.Fatalf("unexpected DBNames: %v", s.DBNames)
	}
	if raw.DBName != "RAW_DATA" {
		t.Fatalf("expected RAW_DATA, got %q", raw.DBName)
	}
}

func TestSchemaColumnNamesNormalized_EmptyDBNames(t *testing.T) {
	s := &schema.Schema{}
	if !schemaColumnNamesNormalized(s) {
		t.Fatal("empty DBNames should be treated as normalized")
	}
}

func TestSchemaColumnNamesNormalized_DetectsLowercase(t *testing.T) {
	s := &schema.Schema{DBNames: []string{"TITLE", "raw_data"}}
	if schemaColumnNamesNormalized(s) {
		t.Fatal("should detect lowercase column in DBNames")
	}
}
