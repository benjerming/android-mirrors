package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	appconfig "assassin-android-controller/internal/application/config"
	appgroup "assassin-android-controller/internal/application/group"
	appinstance "assassin-android-controller/internal/application/instance"
	appuser "assassin-android-controller/internal/application/user"
	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/user"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"
	"assassin-android-controller/internal/interfaces/httpapi"
	"assassin-android-controller/internal/server"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// groupTestDeps 表示分组接口测试所需的全套依赖。
type groupTestDeps struct {
	router         http.Handler
	sessionService *appuser.SessionService
	groupRepo      *infrarepo.GroupRepoGorm
	instanceRepo   *infrarepo.InstanceRepositoryGORM
	userRepo       *infrarepo.UserRepositoryGORM
	configService  *appconfig.Service
}

func newGroupTestDeps(t *testing.T) *groupTestDeps {
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
	groupRepo := infrarepo.NewGroupRepository(db)

	sessionService, err := appuser.NewSessionService(userRepo, "test-secret")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	templateService := appinstance.NewTemplateService(templateRepo)
	instanceService := appinstance.NewInstanceService(instanceRepo, templateRepo)

	abs, err := filepath.Abs("../../../configs/config.json")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	configService, err := appconfig.New(abs)
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	groupService := appgroup.NewService(groupRepo, configService, nil)

	handlerSet := server.HandlerSet{
		Session:  httpapi.NewSessionHandler(sessionService),
		Template: httpapi.NewTemplateHandler(templateService),
		Instance: httpapi.NewInstanceHandler(instanceService),
		Config:   httpapi.NewConfigHandler(configService),
		Group:    httpapi.NewGroupHandler(groupService, configService),
	}

	return &groupTestDeps{
		router:         server.NewRouter(handlerSet, nil, nil),
		sessionService: sessionService,
		groupRepo:      groupRepo,
		instanceRepo:   instanceRepo,
		userRepo:       userRepo,
		configService:  configService,
	}
}

// firstProfileID 从 config.json 拿到首个 profile id，避免硬编码具体取值。
func (d *groupTestDeps) firstProfileID(t *testing.T) string {
	t.Helper()
	opts := d.configService.Options()
	if len(opts.Profiles) == 0 {
		t.Fatal("config has no profiles")
	}
	return opts.Profiles[0].ID
}

func (d *groupTestDeps) firstLanguageCode(t *testing.T) string {
	t.Helper()
	opts := d.configService.Options()
	if len(opts.Languages) == 0 {
		t.Fatal("config has no languages")
	}
	return opts.Languages[0].Code
}

func doJSON(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestGroupHandler_List_RequiresAuth(t *testing.T) {
	deps := newGroupTestDeps(t)
	w := doJSON(t, deps.router, http.MethodGet, "/api/v1/groups", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expect 401, got %d", w.Code)
	}
}

func TestGroupHandler_Create_List_HappyPath(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)

	createBody := map[string]any{
		"name":      "组A",
		"profileId": profileID,
		"languages": []string{lang},
	}
	w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", token, createBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expect 200, got %d body=%s", w.Code, w.Body.String())
	}

	var created struct {
		Group struct {
			ID                 uint   `json:"id"`
			Name               string `json:"name"`
			ProfileID          string `json:"profileId"`
			ProfileDisplayName string `json:"profileDisplayName"`
			AggregateState     string `json:"aggregateState"`
			InstanceCount      int    `json:"instanceCount"`
		} `json:"group"`
		Instances []map[string]any `json:"instances"`
		Failed    []map[string]any `json:"failed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Group.ID == 0 || created.Group.Name != "组A" || created.Group.ProfileID != profileID {
		t.Fatalf("unexpected group: %+v", created.Group)
	}
	if created.Group.AggregateState != "all_stopped" {
		t.Errorf("expect all_stopped (no instances yet), got %q", created.Group.AggregateState)
	}
	if created.Group.ProfileDisplayName == "" {
		t.Errorf("expect profileDisplayName populated")
	}
	if created.Instances == nil {
		t.Errorf("instances should be non-nil empty slice for JSON contract")
	}
	if created.Failed == nil {
		t.Errorf("failed should be non-nil empty slice for JSON contract")
	}

	// List should return that single row.
	listW := doJSON(t, deps.router, http.MethodGet, "/api/v1/groups", token, nil)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status=%d", listW.Code)
	}
	var listed []map[string]any
	if err := json.Unmarshal(listW.Body.Bytes(), &listed); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expect 1 group, got %d", len(listed))
	}
	if listed[0]["aggregateState"] != "all_stopped" {
		t.Errorf("expect all_stopped, got %v", listed[0]["aggregateState"])
	}
}

func TestGroupHandler_Create_DuplicateNameReturns409(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)
	body := map[string]any{"name": "组A", "profileId": profileID, "languages": []string{lang}}

	if w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", token, body); w.Code != http.StatusOK {
		t.Fatalf("first create=%d", w.Code)
	}
	w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", token, body)
	if w.Code != http.StatusConflict {
		t.Fatalf("expect 409, got %d", w.Code)
	}
}

func TestGroupHandler_Create_InvalidProfileReturns400(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	lang := deps.firstLanguageCode(t)

	w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", token, map[string]any{
		"name":      "组A",
		"profileId": "phantom",
		"languages": []string{lang},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

func TestGroupHandler_Create_EmptyLanguagesReturns400(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	profileID := deps.firstProfileID(t)

	w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", token, map[string]any{
		"name":      "组A",
		"profileId": profileID,
		"languages": []string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expect 400, got %d", w.Code)
	}
}

func TestGroupHandler_List_OnlyOwnUser(t *testing.T) {
	deps := newGroupTestDeps(t)
	tokenA := loginAsUser(t, deps.sessionService, "atlas")
	tokenB := loginAsUser(t, deps.sessionService, "bob")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)

	// atlas creates 2, bob creates 1.
	for _, name := range []string{"组A", "组B"} {
		if w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", tokenA, map[string]any{
			"name": name, "profileId": profileID, "languages": []string{lang},
		}); w.Code != http.StatusOK {
			t.Fatalf("seed %s: %d", name, w.Code)
		}
	}
	if w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", tokenB, map[string]any{
		"name": "组X", "profileId": profileID, "languages": []string{lang},
	}); w.Code != http.StatusOK {
		t.Fatalf("seed bob: %d", w.Code)
	}

	w := doJSON(t, deps.router, http.MethodGet, "/api/v1/groups", tokenA, nil)
	var listed []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expect 2 groups for atlas, got %d", len(listed))
	}
}

func TestGroupHandler_List_AggregatesInstanceStats(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	// resolve atlas userID once.
	atlas, err := deps.userRepo.FindByUsername(context.Background(), "atlas")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}

	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)
	w := doJSON(t, deps.router, http.MethodPost, "/api/v1/groups", token, map[string]any{
		"name": "组A", "profileId": profileID, "languages": []string{lang},
	})
	var created struct {
		Group struct {
			ID uint `json:"id"`
		} `json:"group"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	groupID := created.Group.ID

	// Seed 3 instances directly: 2 running, 1 stopped → partial.
	statuses := []emulator.InstanceStatus{
		emulator.InstanceStatusRunning,
		emulator.InstanceStatusRunning,
		emulator.InstanceStatusStopped,
	}
	for i, s := range statuses {
		inst := &emulator.Instance{
			Name:   fmt.Sprintf("u%d__group%d_%d", atlas.ID, groupID, i),
			Mode:   emulator.InstanceModeReusable,
			Status: s,
			UserID: atlas.ID,
		}
		if err := deps.instanceRepo.Create(context.Background(), inst); err != nil {
			t.Fatalf("seed inst: %v", err)
		}
		// Set GroupID + OwnerUID directly via raw update — repo doesn't expose group fields yet.
		if err := deps.groupRepo.AttachInstanceForTest(context.Background(), inst.ID, groupID, atlas.ID); err != nil {
			t.Fatalf("attach: %v", err)
		}
	}

	listW := doJSON(t, deps.router, http.MethodGet, "/api/v1/groups", token, nil)
	if listW.Code != http.StatusOK {
		t.Fatalf("list=%d body=%s", listW.Code, listW.Body.String())
	}
	var listed []map[string]any
	_ = json.Unmarshal(listW.Body.Bytes(), &listed)
	if len(listed) != 1 {
		t.Fatalf("expect 1 row, got %d", len(listed))
	}
	row := listed[0]
	if row["instanceCount"].(float64) != 3 {
		t.Errorf("instanceCount=%v", row["instanceCount"])
	}
	if row["runningCount"].(float64) != 2 {
		t.Errorf("runningCount=%v", row["runningCount"])
	}
	if row["aggregateState"] != "partial" {
		t.Errorf("aggregateState=%v", row["aggregateState"])
	}
}

func TestGroupHandler_Rename_HappyAndConflict(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)

	idA := mustCreateGroup(t, deps.router, token, "组A", profileID, lang)
	mustCreateGroup(t, deps.router, token, "组B", profileID, lang)

	// rename 组A → 组C OK
	w := doJSON(t, deps.router, http.MethodPatch, "/api/v1/groups/"+strconv.FormatUint(uint64(idA), 10), token, map[string]any{"name": "组C"})
	if w.Code != http.StatusOK {
		t.Fatalf("rename ok=%d body=%s", w.Code, w.Body.String())
	}
	// rename 组A (now 组C) → 组B conflict
	w = doJSON(t, deps.router, http.MethodPatch, "/api/v1/groups/"+strconv.FormatUint(uint64(idA), 10), token, map[string]any{"name": "组B"})
	if w.Code != http.StatusConflict {
		t.Fatalf("rename conflict expect 409, got %d", w.Code)
	}
}

func TestGroupHandler_Rename_OtherUserGets404(t *testing.T) {
	deps := newGroupTestDeps(t)
	tokenA := loginAsUser(t, deps.sessionService, "atlas")
	tokenB := loginAsUser(t, deps.sessionService, "bob")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)

	idA := mustCreateGroup(t, deps.router, tokenA, "组A", profileID, lang)
	w := doJSON(t, deps.router, http.MethodPatch, "/api/v1/groups/"+strconv.FormatUint(uint64(idA), 10), tokenB, map[string]any{"name": "组Z"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

func TestGroupHandler_Delete_RemovesGroup(t *testing.T) {
	deps := newGroupTestDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)

	idA := mustCreateGroup(t, deps.router, token, "组A", profileID, lang)

	w := doJSON(t, deps.router, http.MethodDelete, "/api/v1/groups/"+strconv.FormatUint(uint64(idA), 10), token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete=%d", w.Code)
	}
	// next list empty.
	listW := doJSON(t, deps.router, http.MethodGet, "/api/v1/groups", token, nil)
	var listed []map[string]any
	_ = json.Unmarshal(listW.Body.Bytes(), &listed)
	if len(listed) != 0 {
		t.Fatalf("expect 0 after delete, got %d", len(listed))
	}
	// second delete → 404.
	w = doJSON(t, deps.router, http.MethodDelete, "/api/v1/groups/"+strconv.FormatUint(uint64(idA), 10), token, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404 on second delete, got %d", w.Code)
	}
}

func TestGroupHandler_Delete_OtherUserGets404(t *testing.T) {
	deps := newGroupTestDeps(t)
	tokenA := loginAsUser(t, deps.sessionService, "atlas")
	tokenB := loginAsUser(t, deps.sessionService, "bob")
	profileID := deps.firstProfileID(t)
	lang := deps.firstLanguageCode(t)

	idA := mustCreateGroup(t, deps.router, tokenA, "组A", profileID, lang)
	w := doJSON(t, deps.router, http.MethodDelete, "/api/v1/groups/"+strconv.FormatUint(uint64(idA), 10), tokenB, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expect 404, got %d", w.Code)
	}
}

func mustCreateGroup(t *testing.T, router http.Handler, token, name, profileID, lang string) uint {
	t.Helper()
	w := doJSON(t, router, http.MethodPost, "/api/v1/groups", token, map[string]any{
		"name": name, "profileId": profileID, "languages": []string{lang},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create %s: %d body=%s", name, w.Code, w.Body.String())
	}
	var created struct {
		Group struct {
			ID uint `json:"id"`
		} `json:"group"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return created.Group.ID
}

// _ keeps the user import live for tests that may add Body checks later.
var _ = user.User{}
