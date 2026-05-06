package repository

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestAutoMigrate_GroupAndInstanceColumns 验证迁移后 instances 表新增 group_id / language /
// profile_id / owner_uid 等字段，并且 groups 表本身也已建好。
func TestAutoMigrate_GroupAndInstanceColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	mig := db.Migrator()
	for _, col := range []string{"group_id", "language", "profile_id", "owner_uid", "source", "spec_json"} {
		if !mig.HasColumn(&InstanceModel{}, col) {
			t.Errorf("instances missing column %s", col)
		}
	}
	if !mig.HasTable(&GroupModel{}) {
		t.Error("groups table missing")
	}
}
