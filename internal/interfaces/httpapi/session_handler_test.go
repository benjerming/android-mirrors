package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	appinstance "assassin-android-controller/internal/application/instance"
	appuser "assassin-android-controller/internal/application/user"
	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/user"
	"assassin-android-controller/internal/interfaces/httpapi"
	"assassin-android-controller/internal/server"

	infrarepo "assassin-android-controller/internal/infrastructure/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// testHTTPDeps 表示 HTTP 层测试要共用的一组服务和仓储。
type testHTTPDeps struct {
	router          http.Handler
	sessionService  *appuser.SessionService
	instanceService *appinstance.InstanceService
	templateService *appinstance.TemplateService
	userRepo        *infrarepo.UserRepositoryGORM
	templateRepo    *infrarepo.TemplateRepositoryGORM
	instanceRepo    *infrarepo.InstanceRepositoryGORM
}

// newHTTPDeps 用来搭一个轻量的测试路由，方便覆盖 session/template/instance 接口。
func newHTTPDeps(t *testing.T) *testHTTPDeps {
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
		t.Fatalf("new session service: %v", err)
	}
	templateService := appinstance.NewTemplateService(templateRepo)
	instanceService := appinstance.NewInstanceService(instanceRepo, templateRepo)

	handlerSet := server.HandlerSet{
		Session:  httpapi.NewSessionHandler(sessionService),
		Template: httpapi.NewTemplateHandler(templateService),
		Instance: httpapi.NewInstanceHandler(instanceService),
	}

	return &testHTTPDeps{
		router:          server.NewRouter(handlerSet, nil, nil),
		sessionService:  sessionService,
		instanceService: instanceService,
		templateService: templateService,
		userRepo:        userRepo,
		templateRepo:    templateRepo,
		instanceRepo:    instanceRepo,
	}
}

// loginAsUser 用来拿到带鉴权的测试 token，避免每个测试重复拼登录请求。
func loginAsUser(t *testing.T, service *appuser.SessionService, username string) string {
	t.Helper()

	token, err := service.Login(context.Background(), username)
	if err != nil {
		t.Fatalf("login user: %v", err)
	}

	return token
}

// TestSessionHandlerLoginReturnsToken 用来确认登录接口会返回前端需要的 token 字段。
func TestSessionHandlerLoginReturnsToken(t *testing.T) {
	deps := newHTTPDeps(t)

	requestBody, err := json.Marshal(map[string]string{"username": "atlas"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/login", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	deps.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["token"] == "" {
		t.Fatal("expected token in response")
	}
}

// TestSessionHandlerMeReturnsUsername 用来确认会话自检接口会返回当前用户名。
func TestSessionHandlerMeReturnsUsername(t *testing.T) {
	deps := newHTTPDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/session/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	deps.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["username"] != "atlas" {
		t.Fatalf("expected username atlas, got %s", payload["username"])
	}
}

// TestTemplateHandlerListsTemplates 用来确认模板接口会返回建机弹窗够用的最小字段。
func TestTemplateHandlerListsTemplates(t *testing.T) {
	deps := newHTTPDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")

	if err := deps.templateRepo.Create(context.Background(), &emulator.Template{
		Name:        "pixel-8",
		Description: "Pixel 8 API 34",
		SystemImage: "system-images;android-34;google_apis;x86_64",
	}); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	deps.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(payload) != 1 {
		t.Fatalf("expected 1 template, got %d", len(payload))
	}

	if payload[0]["name"] != "pixel-8" {
		t.Fatalf("expected template name pixel-8, got %v", payload[0]["name"])
	}
}

// TestInstanceHandlerListsAndMutatesOwnedInstances 用来确认实例接口能列出自己的实例并支持创建启停删。
func TestInstanceHandlerListsAndMutatesOwnedInstances(t *testing.T) {
	deps := newHTTPDeps(t)
	token := loginAsUser(t, deps.sessionService, "atlas")
	otherUser := &user.User{Username: "other"}
	if err := deps.userRepo.Create(context.Background(), otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}

	template := &emulator.Template{
		Name:        "pixel-8",
		Description: "Pixel 8 API 34",
		SystemImage: "system-images;android-34;google_apis;x86_64",
	}
	if err := deps.templateRepo.Create(context.Background(), template); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	if err := deps.instanceRepo.Create(context.Background(), &emulator.Instance{
		Name:       "u2__other",
		Tag:        "other",
		Mode:       emulator.InstanceModeReusable,
		Status:     emulator.InstanceStatusRunning,
		UserID:     otherUser.ID,
		TemplateID: template.ID,
	}); err != nil {
		t.Fatalf("seed foreign instance: %v", err)
	}

	// B-33（M20）：POST /api/v1/instances 已删除；新建走 POST /api/v1/groups。
	deprecatedBody, _ := json.Marshal(map[string]any{
		"templateId": template.ID, "tag": "pixel8_api34_01", "mode": "reusable",
	})
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/instances", bytes.NewReader(deprecatedBody))
	createRequest.Header.Set("Authorization", "Bearer "+token)
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	deps.router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusNotFound && createRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected POST /instances to be 404/405 after deprecation, got %d", createRecorder.Code)
	}

	// 通过 repo 直接 seed 一台属于 atlas 的实例，模拟分组建机后的状态。
	atlasInst := &emulator.Instance{
		Name:       "u1__pixel8_api34_01",
		Tag:        "pixel8_api34_01",
		Mode:       emulator.InstanceModeReusable,
		Status:     emulator.InstanceStatusStopped,
		UserID:     1,
		TemplateID: template.ID,
	}
	if err := deps.instanceRepo.Create(context.Background(), atlasInst); err != nil {
		t.Fatalf("seed atlas instance: %v", err)
	}
	instanceID := int(atlasInst.ID)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()
	deps.router.ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", listRecorder.Code)
	}

	var listed []map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("expected 1 owned instance, got %d", len(listed))
	}

	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/instances/"+strconv.Itoa(instanceID)+"/start", nil)
	startRequest.Header.Set("Authorization", "Bearer "+token)
	startRecorder := httptest.NewRecorder()
	deps.router.ServeHTTP(startRecorder, startRequest)

	if startRecorder.Code != http.StatusOK {
		t.Fatalf("expected start status 200, got %d", startRecorder.Code)
	}

	stopRequest := httptest.NewRequest(http.MethodPost, "/api/v1/instances/"+strconv.Itoa(instanceID)+"/stop", nil)
	stopRequest.Header.Set("Authorization", "Bearer "+token)
	stopRecorder := httptest.NewRecorder()
	deps.router.ServeHTTP(stopRecorder, stopRequest)

	if stopRecorder.Code != http.StatusOK {
		t.Fatalf("expected stop status 200, got %d", stopRecorder.Code)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/instances/"+strconv.Itoa(instanceID), nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+token)
	deleteRecorder := httptest.NewRecorder()
	deps.router.ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d", deleteRecorder.Code)
	}
}
