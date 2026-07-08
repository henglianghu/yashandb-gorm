package yasdb

import "gorm.io/gorm"

func registerNormalizeSchemaCallbacks(db *gorm.DB) {
	normalize := func(db *gorm.DB) {
		if dialector, ok := db.Dialector.(*Dialector); ok && dialector.Config != nil && dialector.NamingCaseSensitive {
			return
		}
		if dialector, ok := db.Dialector.(Dialector); ok && dialector.Config != nil && dialector.NamingCaseSensitive {
			return
		}
		if db.Statement != nil && db.Statement.Schema != nil {
			NormalizeSchemaColumnNames(db.Statement.Schema)
		}
	}

	_ = db.Callback().Create().Before("gorm:before_create").Register("yasdb:normalize_schema", normalize)
	_ = db.Callback().Query().Before("gorm:query").Register("yasdb:normalize_schema", normalize)
	_ = db.Callback().Update().Before("gorm:before_update").Register("yasdb:normalize_schema", normalize)
	_ = db.Callback().Delete().Before("gorm:before_delete").Register("yasdb:normalize_schema", normalize)
	_ = db.Callback().Row().Before("gorm:row").Register("yasdb:normalize_schema", normalize)
	_ = db.Callback().Raw().Before("gorm:raw").Register("yasdb:normalize_schema", normalize)
}
