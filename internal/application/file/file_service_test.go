package file_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appfile "assassin-android-controller/internal/application/file"
	appinstance "assassin-android-controller/internal/application/instance"
	"assassin-android-controller/internal/domain/emulator"
	domainuser "assassin-android-controller/internal/domain/user"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type fakeADB struct {
	pushes  []string
	removes []string
}

func (f *fakeADB) Push(_ context.Context, _, _, remote string) error {
	f.pushes = append(f.pushes, remote)
	return nil
}

func (f *fakeADB) RemoveRemote(_ context.Context, _, remote string) error {
	f.removes = append(f.removes, remote)
	return nil
}

func newSvc(t *testing.T, allowed string) (*appfile.Service, *fakeADB, uint, uint) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := infrarepo.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	userRepo := infrarepo.NewUserRepositoryGORM(db)
	instanceRepo := infrarepo.NewInstanceRepositoryGORM(db)
	templateRepo := infrarepo.NewTemplateRepositoryGORM(db)
	u := &domainuser.User{Username: "tester"}
	if err := userRepo.Create(context.Background(), u); err != nil {
		t.Fatalf("user: %v", err)
	}
	inst := &emulator.Instance{
		Name:   "u1__test",
		Mode:   emulator.InstanceModeReusable,
		Status: emulator.InstanceStatusRunning,
		UserID: u.ID,
		Serial: "emu5554",
	}
	if err := instanceRepo.Create(context.Background(), inst); err != nil {
		t.Fatalf("inst: %v", err)
	}
	is := appinstance.NewInstanceService(instanceRepo, templateRepo)
	adb := &fakeADB{}
	svc := appfile.New(is, adb, t.TempDir(), allowed)
	return svc, adb, u.ID, inst.ID
}

func TestPush_AllowedPath(t *testing.T) {
	svc, adb, uid, iid := newSvc(t, "/sdcard/Download/")
	if err := svc.Push(context.Background(), uid, iid, "x.bin", "/sdcard/Download/x.bin", strings.NewReader("hello")); err != nil {
		t.Fatalf("push: %v", err)
	}
	if len(adb.pushes) != 1 || adb.pushes[0] != "/sdcard/Download/x.bin" {
		t.Errorf("expect push recorded, got %+v", adb.pushes)
	}
}

func TestPush_RejectOutsideAllowList(t *testing.T) {
	svc, _, uid, iid := newSvc(t, "/sdcard/Download/")
	err := svc.Push(context.Background(), uid, iid, "x.bin", "/etc/passwd", strings.NewReader("x"))
	if err != appfile.ErrInvalidPath {
		t.Errorf("expect ErrInvalidPath, got %v", err)
	}
}

func TestPush_RejectsDotDot(t *testing.T) {
	svc, _, uid, iid := newSvc(t, "/sdcard/Download/")
	err := svc.Push(context.Background(), uid, iid, "x.bin", "/sdcard/Download/../etc/x", strings.NewReader("x"))
	if err != appfile.ErrInvalidPath {
		t.Errorf("expect ErrInvalidPath, got %v", err)
	}
}

func TestDelete_Allowed(t *testing.T) {
	svc, adb, uid, iid := newSvc(t, "/sdcard/Download/")
	if err := svc.Delete(context.Background(), uid, iid, "/sdcard/Download/x.bin"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(adb.removes) != 1 {
		t.Errorf("expect remove recorded, got %+v", adb.removes)
	}
}
