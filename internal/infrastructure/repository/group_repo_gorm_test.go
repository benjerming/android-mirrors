package repository

import (
	"context"
	"errors"
	"testing"

	"assassin-android-controller/internal/domain/group"
	"assassin-android-controller/internal/domain/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每个测试一个独立 :memory: 实例，避免共享状态。
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestGroupRepo_CreateAndList(t *testing.T) {
	db := newTestDB(t)
	repo := NewGroupRepository(db)
	ctx := context.Background()

	g := &group.Group{UserID: 1, Name: "测试组A", ProfileID: "Mirrors_Xperia_XZ"}
	if err := repo.Create(ctx, g); err != nil {
		t.Fatal(err)
	}
	if g.ID == 0 {
		t.Fatal("expect ID assigned")
	}

	got, err := repo.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "测试组A" {
		t.Errorf("unexpected list: %+v", got)
	}
}

func TestGroupRepo_DuplicateNameRejected(t *testing.T) {
	db := newTestDB(t)
	repo := NewGroupRepository(db)
	ctx := context.Background()
	if err := repo.Create(ctx, &group.Group{UserID: 1, Name: "X", ProfileID: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, &group.Group{UserID: 1, Name: "X", ProfileID: "p"}); err == nil {
		t.Error("expect duplicate rejected")
	}
}

func TestGroupRepo_FindAndUpdateAndDelete(t *testing.T) {
	db := newTestDB(t)
	repo := NewGroupRepository(db)
	ctx := context.Background()

	g := &group.Group{UserID: 1, Name: "G1", ProfileID: "p"}
	if err := repo.Create(ctx, g); err != nil {
		t.Fatal(err)
	}

	// FindOwnedByID hit
	got, err := repo.FindOwnedByID(ctx, 1, g.ID)
	if err != nil || got == nil || got.Name != "G1" {
		t.Fatalf("FindOwnedByID = %+v %v", got, err)
	}

	// FindOwnedByID 跨用户应当 404
	if _, err := repo.FindOwnedByID(ctx, 2, g.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expect ErrNotFound for cross-user, got %v", err)
	}

	// FindByName
	if _, err := repo.FindByName(ctx, 1, "G1"); err != nil {
		t.Errorf("FindByName: %v", err)
	}
	if _, err := repo.FindByName(ctx, 1, "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expect ErrNotFound, got %v", err)
	}

	// UpdateName
	if err := repo.UpdateName(ctx, 1, g.ID, "G1-renamed"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByName(ctx, 1, "G1-renamed"); err != nil {
		t.Errorf("renamed not found: %v", err)
	}
	if err := repo.UpdateName(ctx, 1, 9999, "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expect ErrNotFound on missing id, got %v", err)
	}

	// Delete 级联：先放一条该 group 下的 instance
	if err := db.Create(&InstanceModel{Name: "u1_g1_x", OwnerUID: 1, GroupID: g.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, 1, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindOwnedByID(ctx, 1, g.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expect deleted, got %v", err)
	}
	var cnt int64
	db.Model(&InstanceModel{}).Where("group_id = ?", g.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("expect cascade delete instances, remaining=%d", cnt)
	}
	if err := repo.Delete(ctx, 1, g.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expect ErrNotFound on second delete, got %v", err)
	}
}
