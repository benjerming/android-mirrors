package instance_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	appinstance "assassin-android-controller/internal/application/instance"
	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/user"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// blockingRunner 让 fanout 测试能精确控制 runner.Start 何时返回。
type blockingRunner struct {
	startGate chan struct{}
	mu        sync.Mutex
	startErr  map[string]error
	stopErr   map[string]error
	started   []string
	stopped   []string
}

func newBlockingRunner() *blockingRunner {
	return &blockingRunner{
		startGate: make(chan struct{}),
		startErr:  map[string]error{},
		stopErr:   map[string]error{},
	}
}

func (b *blockingRunner) Start(_ context.Context, name string) (string, error) {
	<-b.startGate
	b.mu.Lock()
	b.started = append(b.started, name)
	err := b.startErr[name]
	b.mu.Unlock()
	if err != nil {
		return "", err
	}
	return "emulator-" + name, nil
}

func (b *blockingRunner) Stop(_ context.Context, serial string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = append(b.stopped, serial)
	return b.stopErr[serial]
}

// seedGroupInstance 直接落一个带 group_id+owner_uid 的实例行，省去建 group_models 表。
// instance_service 的 DispatchStartByGroup 只读 instance_models，不依赖 group 校验。
func seedGroupInstance(t *testing.T, repo *infrarepo.InstanceRepositoryGORM, userID, groupID uint, name string, status emulator.InstanceStatus) *emulator.Instance {
	t.Helper()
	inst := &emulator.Instance{
		Name:     name,
		Tag:      name,
		Mode:     emulator.InstanceModeReusable,
		Status:   status,
		UserID:   userID,
		OwnerUID: userID,
		GroupID:  groupID,
		Source:   "group",
	}
	if err := repo.Create(context.Background(), inst); err != nil {
		t.Fatalf("seed instance %s: %v", name, err)
	}
	return inst
}

// instanceServiceDeps 表示实例服务测试里会反复复用的一组依赖。
type instanceServiceDeps struct {
	service      *appinstance.InstanceService
	templateRepo *infrarepo.TemplateRepositoryGORM
	instanceRepo *infrarepo.InstanceRepositoryGORM
	userRepo     *infrarepo.UserRepositoryGORM
}

// newInstanceServiceForTest 用来准备实例服务测试所需的数据库、仓储和服务。
func newInstanceServiceForTest(t *testing.T) *instanceServiceDeps {
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

	return &instanceServiceDeps{
		service:      appinstance.NewInstanceService(instanceRepo, templateRepo),
		templateRepo: templateRepo,
		instanceRepo: instanceRepo,
		userRepo:     userRepo,
	}
}

// seedUser 用来快速插入测试用户。
func seedUser(t *testing.T, repo *infrarepo.UserRepositoryGORM, username string) *user.User {
	t.Helper()

	record := &user.User{Username: username}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create user: %v", err)
	}

	return record
}

// seedTemplate 用来快速插入测试模板。
func seedTemplate(t *testing.T, repo *infrarepo.TemplateRepositoryGORM) *emulator.Template {
	t.Helper()

	record := &emulator.Template{
		Name:        "pixel-8",
		Description: "Pixel 8 API 34",
		SystemImage: "system-images;android-34;google_apis;x86_64",
	}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create template: %v", err)
	}

	return record
}

// TestInstanceServiceListInstancesOnlyReturnsOwnedInstances 用来确认列表只返回当前用户自己的实例。
func TestInstanceServiceListInstancesOnlyReturnsOwnedInstances(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	owner := seedUser(t, deps.userRepo, "atlas")
	other := seedUser(t, deps.userRepo, "other")
	template := seedTemplate(t, deps.templateRepo)

	if err := deps.instanceRepo.Create(context.Background(), &emulator.Instance{Name: "u1__a", Tag: "a", Mode: emulator.InstanceModeReusable, Status: emulator.InstanceStatusRunning, UserID: owner.ID, TemplateID: template.ID}); err != nil {
		t.Fatalf("create owner instance: %v", err)
	}
	if err := deps.instanceRepo.Create(context.Background(), &emulator.Instance{Name: "u2__b", Tag: "b", Mode: emulator.InstanceModeReusable, Status: emulator.InstanceStatusRunning, UserID: other.ID, TemplateID: template.ID}); err != nil {
		t.Fatalf("create other instance: %v", err)
	}

	items, err := deps.service.ListByUser(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(items))
	}

	if items[0].UserID != owner.ID {
		t.Fatalf("expected instance owned by %d, got %d", owner.ID, items[0].UserID)
	}
}

// TestInstanceServiceCreateInstanceUsesTemplateAndOwnership 用来确认建机请求会写入模板、归属和规范化名称。
func TestInstanceServiceCreateInstanceUsesTemplateAndOwnership(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	owner := seedUser(t, deps.userRepo, "atlas")
	template := seedTemplate(t, deps.templateRepo)

	created, err := deps.service.Create(context.Background(), owner.ID, appinstance.CreateInstanceInput{
		TemplateID: template.ID,
		Tag:        "pixel8_api34_01",
		Mode:       emulator.InstanceModeReusable,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if created.Name != "u1__pixel8_api34_01" {
		t.Fatalf("expected generated name u1__pixel8_api34_01, got %s", created.Name)
	}

	if created.TemplateID != template.ID {
		t.Fatalf("expected template id %d, got %d", template.ID, created.TemplateID)
	}
}

// TestInstanceServiceStartStopDeleteChecksOwnership 用来确认启停删前都会先做实例归属校验。
func TestInstanceServiceStartStopDeleteChecksOwnership(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	owner := seedUser(t, deps.userRepo, "atlas")
	other := seedUser(t, deps.userRepo, "other")
	template := seedTemplate(t, deps.templateRepo)

	instance := &emulator.Instance{
		Name:       "u1__pixel8",
		Tag:        "pixel8",
		Mode:       emulator.InstanceModeReusable,
		Status:     emulator.InstanceStatusStopped,
		UserID:     owner.ID,
		TemplateID: template.ID,
	}
	if err := deps.instanceRepo.Create(context.Background(), instance); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if err := deps.service.Start(context.Background(), other.ID, instance.ID); err == nil {
		t.Fatal("expected start ownership error")
	}

	if err := deps.service.Start(context.Background(), owner.ID, instance.ID); err != nil {
		t.Fatalf("start instance: %v", err)
	}

	started, err := deps.instanceRepo.FindOwnedByID(context.Background(), owner.ID, instance.ID)
	if err != nil {
		t.Fatalf("find started instance: %v", err)
	}
	if started.Status != emulator.InstanceStatusRunning {
		t.Fatalf("expected running status, got %s", started.Status)
	}

	if err := deps.service.Stop(context.Background(), owner.ID, instance.ID); err != nil {
		t.Fatalf("stop instance: %v", err)
	}

	stopped, err := deps.instanceRepo.FindOwnedByID(context.Background(), owner.ID, instance.ID)
	if err != nil {
		t.Fatalf("find stopped instance: %v", err)
	}
	if stopped.Status != emulator.InstanceStatusStopped {
		t.Fatalf("expected stopped status, got %s", stopped.Status)
	}

	if err := deps.service.Delete(context.Background(), owner.ID, instance.ID); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	if _, err := deps.instanceRepo.FindOwnedByID(context.Background(), owner.ID, instance.ID); err == nil {
		t.Fatal("expected deleted instance lookup to fail")
	}
}

func TestDispatchStartByGroup_ImmediatelyMarksStarting(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	runner := newBlockingRunner()
	deps.service.WithRuntime(nil, runner, nil)

	owner := seedUser(t, deps.userRepo, "u1")
	a := seedGroupInstance(t, deps.instanceRepo, owner.ID, 1, "u1__a", emulator.InstanceStatusStopped)
	b := seedGroupInstance(t, deps.instanceRepo, owner.ID, 1, "u1__b", emulator.InstanceStatusStopped)

	transitioning, skipped, err := deps.service.DispatchStartByGroup(context.Background(), owner.ID, 1)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(transitioning) != 2 || len(skipped) != 0 {
		t.Fatalf("dispatch result: transitioning=%v skipped=%v", transitioning, skipped)
	}

	for _, id := range []uint{a.ID, b.ID} {
		got, err := deps.instanceRepo.FindByID(context.Background(), id)
		if err != nil {
			t.Fatalf("lookup %d: %v", id, err)
		}
		if got.Status != emulator.InstanceStatusStarting {
			t.Errorf("instance %d status = %q, want starting", id, got.Status)
		}
	}
}

func TestRunStartFanout_SuccessSetsRunning(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	runner := newBlockingRunner()
	deps.service.WithRuntime(nil, runner, nil)

	owner := seedUser(t, deps.userRepo, "u1")
	a := seedGroupInstance(t, deps.instanceRepo, owner.ID, 1, "u1__a", emulator.InstanceStatusStopped)

	ids, _, err := deps.service.DispatchStartByGroup(context.Background(), owner.ID, 1)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	done := make(chan struct{})
	go func() {
		deps.service.RunStartFanout(context.Background(), ids)
		close(done)
	}()
	close(runner.startGate)
	<-done

	got, err := deps.instanceRepo.FindByID(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.Status != emulator.InstanceStatusRunning {
		t.Errorf("status = %q, want running", got.Status)
	}
	if got.Serial != "emulator-u1__a" {
		t.Errorf("serial = %q, want emulator-u1__a", got.Serial)
	}
}

func TestRunStartFanout_RunnerErrorSetsError(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	runner := newBlockingRunner()
	runner.startErr["u1__a"] = errors.New("boot timeout")
	deps.service.WithRuntime(nil, runner, nil)

	owner := seedUser(t, deps.userRepo, "u1")
	a := seedGroupInstance(t, deps.instanceRepo, owner.ID, 1, "u1__a", emulator.InstanceStatusStopped)

	ids, _, err := deps.service.DispatchStartByGroup(context.Background(), owner.ID, 1)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	done := make(chan struct{})
	go func() {
		deps.service.RunStartFanout(context.Background(), ids)
		close(done)
	}()
	close(runner.startGate)
	<-done

	got, err := deps.instanceRepo.FindByID(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.Status != emulator.InstanceStatusError {
		t.Errorf("status = %q, want error", got.Status)
	}
}

func TestDispatchStartByGroup_SkipsAlreadyStarting(t *testing.T) {
	deps := newInstanceServiceForTest(t)
	runner := newBlockingRunner()
	deps.service.WithRuntime(nil, runner, nil)

	owner := seedUser(t, deps.userRepo, "u1")
	seedGroupInstance(t, deps.instanceRepo, owner.ID, 1, "u1__a", emulator.InstanceStatusStopped)

	first, _, err := deps.service.DispatchStartByGroup(context.Background(), owner.ID, 1)
	if err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first dispatch transitioning: %v", first)
	}
	second, skipped, err := deps.service.DispatchStartByGroup(context.Background(), owner.ID, 1)
	if err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if len(second) != 0 || len(skipped) != 1 {
		t.Errorf("second dispatch: transitioning=%v skipped=%v, want []/1", second, skipped)
	}
}
