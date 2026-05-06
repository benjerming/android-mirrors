package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	appconfig "assassin-android-controller/internal/application/config"
	appinstance "assassin-android-controller/internal/application/instance"
	appuser "assassin-android-controller/internal/application/user"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"
	"assassin-android-controller/internal/infrastructure/scrcpy"
	"assassin-android-controller/internal/interfaces/httpapi"
	"assassin-android-controller/internal/server"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type scrcpyDebugTestDeps struct {
	router         http.Handler
	sessionService *appuser.SessionService
}

func newScrcpyDebugTestDeps(t *testing.T, sm *scrcpy.SessionManager) *scrcpyDebugTestDeps {
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
	templateRepo := infrarepo.NewTemplateRepositoryGORM(db)
	instanceRepo := infrarepo.NewInstanceRepositoryGORM(db)

	sessionService, err := appuser.NewSessionService(userRepo, "test-secret")
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	templateService := appinstance.NewTemplateService(templateRepo)
	instanceService := appinstance.NewInstanceService(instanceRepo, templateRepo)

	abs, err := filepath.Abs("../../../configs/config.json")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	configService, err := appconfig.New(abs)
	if err != nil {
		t.Fatalf("config service: %v", err)
	}

	handlerSet := server.HandlerSet{
		Session:     httpapi.NewSessionHandler(sessionService),
		Template:    httpapi.NewTemplateHandler(templateService),
		Instance:    httpapi.NewInstanceHandler(instanceService),
		Config:      httpapi.NewConfigHandler(configService),
		ScrcpyDebug: httpapi.NewScrcpyDebugHandler(sm),
	}

	return &scrcpyDebugTestDeps{
		router:         server.NewRouter(handlerSet, nil, nil),
		sessionService: sessionService,
	}
}

func TestScrcpyDebugHandler_List_Unauthorized(t *testing.T) {
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{})
	defer sm.Shutdown()

	deps := newScrcpyDebugTestDeps(t, sm)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/scrcpy/sessions", nil)
	deps.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("response missing 'error' field: %s", w.Body.String())
	}
}

func TestScrcpyDebugHandler_List_Empty(t *testing.T) {
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{})
	defer sm.Shutdown()

	deps := newScrcpyDebugTestDeps(t, sm)
	token := loginAsUser(t, deps.sessionService, "atlas")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/scrcpy/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	deps.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	sessions, ok := body["sessions"]
	if !ok {
		t.Fatalf("response missing 'sessions' field: %s", w.Body.String())
	}
	// Snapshots() returns nil when empty; JSON encodes nil slice as null.
	// Both null and [] are acceptable empty representations for zero supervisors.
	if sessions != nil {
		arr, ok := sessions.([]any)
		if !ok {
			t.Errorf("'sessions' is not an array: %T %v", sessions, sessions)
		} else if len(arr) != 0 {
			t.Errorf("expected empty sessions, got %d", len(arr))
		}
	}
}

func TestScrcpyDebugHandler_Reset_NotFound(t *testing.T) {
	sm := scrcpy.NewSessionManager(scrcpy.SessionManagerOpts{})
	defer sm.Shutdown()

	deps := newScrcpyDebugTestDeps(t, sm)
	token := loginAsUser(t, deps.sessionService, "atlas")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/debug/scrcpy/sessions/unknown-serial/reset", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	deps.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404; body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("response missing 'error' field: %s", w.Body.String())
	}
}
