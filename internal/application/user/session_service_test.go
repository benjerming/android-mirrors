package user_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appuser "assassin-android-controller/internal/application/user"
	"assassin-android-controller/internal/domain/user"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newSessionServiceForTest 用来准备会话服务测试需要的数据库和仓储。
func newSessionServiceForTest(t *testing.T) (*appuser.SessionService, *infrarepo.UserRepositoryGORM) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := infrarepo.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	userRepo := infrarepo.NewUserRepositoryGORM(db)

	service, err := appuser.NewSessionService(userRepo, "test-secret")
	if err != nil {
		t.Fatalf("new session service: %v", err)
	}

	return service, userRepo
}

// TestSessionServiceLoginCreatesUserAndReturnsToken 用来确认首次登录会自动建用户并签发 token。
func TestSessionServiceLoginCreatesUserAndReturnsToken(t *testing.T) {
	service, userRepo := newSessionServiceForTest(t)

	token, err := service.Login(context.Background(), "atlas")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	savedUser, err := userRepo.FindByUsername(context.Background(), "atlas")
	if err != nil {
		t.Fatalf("find by username: %v", err)
	}

	if savedUser.Username != "atlas" {
		t.Fatalf("expected username atlas, got %s", savedUser.Username)
	}
}

// TestSessionServiceLoginReusesExistingUser 用来确认同名再次登录不会重复创建用户。
func TestSessionServiceLoginReusesExistingUser(t *testing.T) {
	service, userRepo := newSessionServiceForTest(t)

	seedUser := &user.User{
		Username: "atlas",
	}
	if err := userRepo.Create(context.Background(), seedUser); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	token, err := service.Login(context.Background(), "atlas")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}

	profile, err := service.GetProfile(context.Background(), token)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	if profile.ID != seedUser.ID {
		t.Fatalf("expected reused user id %d, got %d", seedUser.ID, profile.ID)
	}
}

// TestSessionServiceGetProfileRejectsInvalidToken 用来确认无效 token 不会被当成有效会话。
func TestSessionServiceGetProfileRejectsInvalidToken(t *testing.T) {
	service, _ := newSessionServiceForTest(t)

	if _, err := service.GetProfile(context.Background(), "bad-token"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
