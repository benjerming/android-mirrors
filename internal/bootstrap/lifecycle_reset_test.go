package bootstrap

import (
	"context"
	"testing"
	"time"

	"assassin-android-controller/internal/domain/emulator"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := infrarepo.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestResetAll_RewritesAllNonStoppedRowsToStopped(t *testing.T) {
	db := openTestDB(t)
	repo := infrarepo.NewInstanceRepositoryGORM(db)

	rows := []emulator.Instance{
		{Name: "u1__a", Tag: "a", Mode: emulator.InstanceModeReusable, Status: emulator.InstanceStatusRunning, UserID: 1, OwnerUID: 1, GroupID: 1, Serial: "emulator-5554"},
		{Name: "u1__b", Tag: "b", Mode: emulator.InstanceModeReusable, Status: emulator.InstanceStatusStarting, UserID: 1, OwnerUID: 1, GroupID: 1, Serial: ""},
		{Name: "u1__c", Tag: "c", Mode: emulator.InstanceModeReusable, Status: emulator.InstanceStatusError, UserID: 1, OwnerUID: 1, GroupID: 1, Serial: "emulator-5556"},
		{Name: "u1__d", Tag: "d", Mode: emulator.InstanceModeReusable, Status: emulator.InstanceStatusStopped, UserID: 1, OwnerUID: 1, GroupID: 1, Serial: ""},
	}
	for i := range rows {
		if err := repo.Create(context.Background(), &rows[i]); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// adbBin = "" 表示 adb 不可用；ResetAll 必须不 panic 并继续执行 DB 重置。
	if err := ResetAll(ctx, db, "", nil); err != nil {
		t.Fatalf("ResetAll: %v", err)
	}

	for _, name := range []string{"u1__a", "u1__b", "u1__c", "u1__d"} {
		var got struct {
			Status string
			Serial string
		}
		if err := db.Table("instance_models").Select("status, serial").Where("name = ?", name).Take(&got).Error; err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if got.Status != "stopped" || got.Serial != "" {
			t.Errorf("row %s: got status=%q serial=%q, want stopped/empty", name, got.Status, got.Serial)
		}
	}
}
