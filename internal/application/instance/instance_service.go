// Package instance 实现"实例 / 模板"相关的应用服务。
//
// 它处于领域和接口之间：把 HTTP handler 收到的请求翻译成对仓储的调用，
// 同时承担"模板存在性、实例归属、模式合法性"等业务校验，让 handler 只关心序列化和状态码。
package instance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/profile"
	"assassin-android-controller/internal/domain/repository"
	"assassin-android-controller/pkg/logger"
)

var (
	// ErrInvalidTag 表示建机标签为空，没法生成稳定实例名。
	ErrInvalidTag = errors.New("instance: invalid tag")
	// ErrInvalidMode 表示 mode 不是当前后端支持的 3 种模式之一。
	ErrInvalidMode = errors.New("instance: invalid mode")
	// ErrForbidden 表示用户尝试操作不属于自己的实例。
	ErrForbidden = errors.New("instance: forbidden")
	// ErrTemplateNotFound 表示创建实例时引用的模板不存在。
	ErrTemplateNotFound = errors.New("instance: template not found")
	// ErrInstanceNotFound 表示请求里的实例不存在或已经删除。
	ErrInstanceNotFound = errors.New("instance: instance not found")
	// ErrAVDCreate 表示 avdmanager 真实创建 AVD 时失败，会带上底层 stderr 让排查更容易。
	ErrAVDCreate = errors.New("instance: avd create failed")
)

// CreateAVDSpec 表示 AVD 创建所需的硬件 / 系统镜像参数，用来在应用层和基础设施层之间传递。
type CreateAVDSpec struct {
	SystemImage string // SystemImage 表示底层 system image，如 system-images;android-34;...
	Device      string // Device 表示 avdmanager 的设备型号，如 small_phone。
	Resolution  string // Resolution 表示模拟器屏幕分辨率，如 1080x2400。
	Density     int    // Density 表示屏幕像素密度（DPI）。
}

// AVDManager 表示真正去创建 / 删除 AVD 目录的能力，应用服务通过它把"建机"落到底层。
//
// 抽象成接口的目的是让单元测试可以替换成 fake，同时把 avdmanager CLI 的细节限制在 infrastructure 包里。
type AVDManager interface {
	Create(ctx context.Context, instanceName string, spec CreateAVDSpec) error
	Delete(ctx context.Context, instanceName string) error
}

// EmulatorRunner 表示真正去启动 / 停止模拟器进程的能力，应用服务通过它把"开关机"落到底层。
type EmulatorRunner interface {
	Start(ctx context.Context, instanceName string) (serial string, err error)
	Stop(ctx context.Context, serial string) error
}

// ProfileResolver 把 profileID → CreateAVDSpec 的映射抽象出来，
// 由 application/config.Service 提供真实实现，单测可以注入 fake。
type ProfileResolver interface {
	Profile(id string) (profile.Profile, bool)
}

// CreateInstanceInput 表示创建实例时的最小业务输入，正好对应阶段 1 的前端需求。
type CreateInstanceInput struct {
	TemplateID uint                  // TemplateID 表示用户选择的模板编号。
	Tag        string                // Tag 表示用户输入的实例标签，用来生成实例名。
	Mode       emulator.InstanceMode // Mode 表示实例保留模式。
}

// InstanceService 表示实例应用服务，负责列表、创建、启停和删除等阶段 1 动作。
type InstanceService struct {
	instanceRepo repository.InstanceRepository // instanceRepo 用来持久化实例数据和归属关系。
	templateRepo repository.TemplateRepository // templateRepo 用来校验建机时选择的模板是否存在。
	avd          AVDManager                    // avd 用来在底层真正创建/删除 AVD 目录，未注入时退化为 noop。
	runner       EmulatorRunner                // runner 用来在底层真正开关模拟器进程，未注入时退化为 noop。
	profiles     ProfileResolver               // profiles 用来把 profileID → AVD spec，分组建机时使用。
	logger       *slog.Logger                  // logger 用来打印底层命令的错误，避免悄无声息地失败。

	writeLocksMu sync.Mutex          // writeLocksMu 保护 writeLocks 的并发访问。
	writeLocks   map[uint]*sync.Mutex // writeLocks 给每个 instanceID 一把锁，spec §4.5：同实例写操作必须串行。
}

// NewInstanceService 用来创建实例服务，通常在依赖装配阶段调用。
//
// 默认装配的是 noop 的 AVDManager 与 EmulatorRunner，保留给单元测试或没有 Android SDK 的环境使用；
// 真正部署时请通过 WithRuntime 注入 infrastructure 包提供的真实实现。
func NewInstanceService(instanceRepo repository.InstanceRepository, templateRepo repository.TemplateRepository) *InstanceService {
	return &InstanceService{
		instanceRepo: instanceRepo,
		templateRepo: templateRepo,
		avd:          noopAVDManager{},
		runner:       noopEmulatorRunner{},
		writeLocks:   make(map[uint]*sync.Mutex),
	}
}

// LockInstance 拿到指定实例的写操作锁，调用方必须 defer 解锁后返回。
//
// spec §4.5："同一实例上的写操作必须串行"——控制 / 安装 / 卸载 / 清缓存 / 文件上传都
// 应当先调用 LockInstance；多个 instanceID 之间锁互不影响。
func (s *InstanceService) LockInstance(instanceID uint) func() {
	s.writeLocksMu.Lock()
	mu, ok := s.writeLocks[instanceID]
	if !ok {
		mu = &sync.Mutex{}
		s.writeLocks[instanceID] = mu
	}
	s.writeLocksMu.Unlock()
	mu.Lock()
	return mu.Unlock
}

// WithRuntime 用来在装配阶段把真实的 AVDManager 和 EmulatorRunner 注入到服务里。
//
// 设计成"事后注入"而非构造参数，是为了保留 NewInstanceService 的极简签名，
// 让现有单元测试不必关心底层依赖；生产装配链只多一行 .WithRuntime(...) 即可。
func (s *InstanceService) WithRuntime(avd AVDManager, runner EmulatorRunner, logger *slog.Logger) *InstanceService {
	if avd != nil {
		s.avd = avd
	}
	if runner != nil {
		s.runner = runner
	}
	if logger != nil {
		s.logger = logger
	}
	return s
}

// WithProfiles 用来注入 profileID → AVD spec 的解析器，分组建机走 CreateForGroup 时必须设置。
func (s *InstanceService) WithProfiles(p ProfileResolver) *InstanceService {
	s.profiles = p
	return s
}

// ListByUser 用来列出当前用户自己的实例，避免把别人的实例暴露给前端。
func (s *InstanceService) ListByUser(ctx context.Context, userID uint) ([]emulator.Instance, error) {
	return s.instanceRepo.ListByUserID(ctx, userID)
}

// Create 用来按"模板 + tag + mode"创建实例，并把底层 AVD 目录一并建出来。
//
// 落地顺序：
//  1. 校验 tag / mode / 模板存在性；
//  2. 调用底层 avdmanager 创建真实 AVD（这是用户反馈的关键缺失环节）；
//  3. 把实例记录写入数据库；如果数据库写入失败，会把刚才建出来的 AVD 删除回滚，避免残留。
func (s *InstanceService) Create(ctx context.Context, userID uint, input CreateInstanceInput) (*emulator.Instance, error) {
	tag := strings.TrimSpace(input.Tag)
	if tag == "" {
		return nil, ErrInvalidTag
	}

	if !isValidMode(input.Mode) {
		return nil, ErrInvalidMode
	}

	template, err := s.templateRepo.FindByID(ctx, input.TemplateID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	instanceName := fmt.Sprintf("u%d__%s", userID, tag)

	spec := CreateAVDSpec{
		SystemImage: template.SystemImage,
		Device:      template.Device,
		Resolution:  template.Resolution,
		Density:     template.Density,
	}
	s.logInfo(ctx, "creating instance", "instance", instanceName, "template_id", input.TemplateID, "user_id", userID, "mode", string(input.Mode))

	if err := s.avd.Create(ctx, instanceName, spec); err != nil {
		s.logError(ctx, "avd create failed", "instance", instanceName, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrAVDCreate, err)
	}

	instance := &emulator.Instance{
		Name:       instanceName,
		Tag:        tag,
		Mode:       input.Mode,
		Status:     emulator.InstanceStatusStopped,
		UserID:     userID,
		TemplateID: input.TemplateID,
	}

	if err := s.instanceRepo.Create(ctx, instance); err != nil {
		// 数据库失败时尝试回滚已经建好的 AVD，避免占着磁盘但没记录可管理。
		// 用 context.Background() 是怕原 ctx 已经被取消，回滚动作还是要执行；
		// 但用 WithRequestID 把原始 request_id 续上，让回滚日志能和创建日志串起来。
		rollbackCtx := logger.WithRequestID(context.Background(), logger.RequestIDFromContext(ctx))
		if delErr := s.avd.Delete(rollbackCtx, instanceName); delErr != nil {
			s.logError(rollbackCtx, "avd rollback after db failure failed", "instance", instanceName, "error", delErr)
		}
		return nil, err
	}

	s.logInfo(ctx, "instance created", "instance", instanceName, "instance_id", instance.ID)
	return instance, nil
}

// Start 用来启动当前用户的实例，把底层模拟器进程拉起来并记录 adb 序列号。
func (s *InstanceService) Start(ctx context.Context, userID uint, instanceID uint) error {
	instance, err := s.loadOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}

	s.logInfo(ctx, "starting instance", "instance", instance.Name, "instance_id", instance.ID)

	serial, err := s.runner.Start(ctx, instance.Name)
	if err != nil {
		s.logError(ctx, "emulator start failed", "instance", instance.Name, "error", err)
		return err
	}

	instance.Status = emulator.InstanceStatusRunning
	instance.Serial = serial

	s.logInfo(ctx, "instance started", "instance", instance.Name, "serial", serial)
	return s.instanceRepo.Update(ctx, instance)
}

// Stop 用来停止当前用户的实例，关掉底层模拟器进程并清空序列号。
func (s *InstanceService) Stop(ctx context.Context, userID uint, instanceID uint) error {
	instance, err := s.loadOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}

	s.logInfo(ctx, "stopping instance", "instance", instance.Name, "instance_id", instance.ID, "serial", instance.Serial)

	if instance.Serial != "" {
		if err := s.runner.Stop(ctx, instance.Serial); err != nil {
			s.logError(ctx, "emulator stop failed", "serial", instance.Serial, "error", err)
		}
	}

	instance.Status = emulator.InstanceStatusStopped
	instance.Serial = ""

	s.logInfo(ctx, "instance stopped", "instance", instance.Name)
	return s.instanceRepo.Update(ctx, instance)
}

// Delete 用来删除当前用户的实例：先停模拟器，再删 AVD 目录，最后删数据库记录。
func (s *InstanceService) Delete(ctx context.Context, userID uint, instanceID uint) error {
	instance, err := s.loadOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}

	s.logInfo(ctx, "deleting instance", "instance", instance.Name, "instance_id", instance.ID)

	if instance.Serial != "" {
		if err := s.runner.Stop(ctx, instance.Serial); err != nil {
			s.logError(ctx, "emulator stop on delete failed", "serial", instance.Serial, "error", err)
		}
	}

	if err := s.avd.Delete(ctx, instance.Name); err != nil {
		s.logError(ctx, "avd delete failed", "instance", instance.Name, "error", err)
		// 这里不阻塞数据库删除：避免 AVD 目录被人手工删了之后，前端永远删不掉这条记录。
	}

	if err := s.instanceRepo.Delete(ctx, userID, instanceID); err != nil {
		s.logError(ctx, "instance db delete failed", "instance", instance.Name, "error", err)
		return err
	}

	s.logInfo(ctx, "instance deleted", "instance", instance.Name)
	return nil
}

// CreateForGroup 给分组建一台实例：先建 AVD，再写库；DB 失败时回滚 AVD。
//
// 实例名约定：u{uid}_g{gid}_{profileID}_{language}。重复调用同一分组+语言会让 AVD
// 命令端报已存在，从而被上层映射成 ErrAVDCreate；这是预期行为，由调用方决定是否兜底。
func (s *InstanceService) CreateForGroup(ctx context.Context, userID, groupID uint, profileID, language string) (uint, error) {
	if s.profiles == nil {
		return 0, fmt.Errorf("%w: missing ProfileResolver", ErrAVDCreate)
	}
	p, ok := s.profiles.Profile(profileID)
	if !ok {
		return 0, fmt.Errorf("%w: unknown profile %q", ErrAVDCreate, profileID)
	}

	name := fmt.Sprintf("u%d_g%d_%s_%s", userID, groupID, profileID, language)
	spec := CreateAVDSpec{
		SystemImage: profileSystemImage(p),
		Device:      p.Device,
		Resolution:  p.Resolution,
		Density:     p.Density,
	}

	s.logInfo(ctx, "creating group instance", "instance", name, "user_id", userID, "group_id", groupID, "language", language)
	if err := s.avd.Create(ctx, name, spec); err != nil {
		s.logError(ctx, "avd create failed", "instance", name, "error", err)
		return 0, fmt.Errorf("%w: %v", ErrAVDCreate, err)
	}

	inst := emulator.Instance{
		Name:      name,
		Mode:      emulator.InstanceModeReusable,
		Status:    emulator.InstanceStatusStopped,
		UserID:    userID,
		OwnerUID:  userID,
		GroupID:   groupID,
		Language:  language,
		ProfileID: profileID,
		Source:    "group",
	}
	if err := s.instanceRepo.Create(ctx, &inst); err != nil {
		rollbackCtx := logger.WithRequestID(context.Background(), logger.RequestIDFromContext(ctx))
		if delErr := s.avd.Delete(rollbackCtx, name); delErr != nil {
			s.logError(rollbackCtx, "avd rollback after db failure failed", "instance", name, "error", delErr)
		}
		return 0, err
	}
	return inst.ID, nil
}

// profileSystemImage 给分组实例选一个 system image。
//
// 当前 Profile 值对象不带 SystemImage 字段（spec §6.2 把它收在 config.json 全局），
// 这里先返回与现有种子模板一致的固定值；B-18/B-21 阶段如果引入 per-profile 镜像，再回来扩展。
func profileSystemImage(_ profile.Profile) string {
	return "system-images;android-34;google_apis;x86_64"
}

// DispatchStartByGroup 是整组启动的"派发"阶段：把所有"非 running/非 starting"
// 的实例行翻成 'starting'，并返回 (transitioning, skipped) 两段 id 列表。返回后调用方
// 必须再调 RunStartFanout(transitioning) 真正去拉模拟器；这两步分离保证 HTTP handler 能
// 立即 202 返回，不被 boot 阻塞。
//
// transitioning：本次刚被翻成 starting 的；调用方负责 fanout。
// skipped：当前已在 running / starting，幂等跳过。
func (s *InstanceService) DispatchStartByGroup(ctx context.Context, userID, groupID uint) (transitioning []uint, skipped []uint, err error) {
	rows, err := s.instanceRepo.ListByGroup(ctx, userID, groupID)
	if err != nil {
		s.logError(ctx, "dispatch start: list failed", "user_id", userID, "group_id", groupID, "error", err)
		return nil, nil, err
	}
	for _, inst := range rows {
		if inst.Status == emulator.InstanceStatusRunning || inst.Status == emulator.InstanceStatusStarting {
			skipped = append(skipped, inst.ID)
			continue
		}
		updated := inst
		updated.Status = emulator.InstanceStatusStarting
		updated.Serial = ""
		if err := s.instanceRepo.Update(ctx, &updated); err != nil {
			s.logError(ctx, "dispatch start: mark starting failed", "instance_id", inst.ID, "error", err)
			return nil, nil, err
		}
		transitioning = append(transitioning, inst.ID)
	}
	s.logInfo(ctx, "group start dispatched", "user_id", userID, "group_id", groupID, "transitioning", len(transitioning), "skipped", len(skipped))
	return transitioning, skipped, nil
}

// RunStartFanout 是整组启动的"执行"阶段，调用方应在 goroutine 里跑（用 context.Background()
// 衍生的 ctx，不要传 HTTP 请求 ctx —— 客户端断开不应取消已派发的启动）。
//
// 每个 id：runner.Start 成功 → status=running + serial；失败 → status=error；DB 写失败仅日志。
// panic 由 defer recover 兜底，对应行落 error 状态。
func (s *InstanceService) RunStartFanout(ctx context.Context, ids []uint) {
	if len(ids) == 0 {
		return
	}
	const concurrency = 8
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					s.logError(ctx, "fanout start panic", "instance_id", id, "panic", r)
					s.markInstanceError(ctx, id)
				}
			}()
			inst, err := s.instanceRepo.FindByID(ctx, id)
			if err != nil || inst == nil {
				s.logError(ctx, "fanout start: instance not found", "instance_id", id, "error", err)
				return
			}
			serial, err := s.runner.Start(ctx, inst.Name)
			if err != nil {
				s.logError(ctx, "fanout start: runner failed", "instance_id", id, "instance", inst.Name, "error", err)
				s.markInstanceError(ctx, id)
				return
			}
			inst.Status = emulator.InstanceStatusRunning
			inst.Serial = serial
			if err := s.instanceRepo.Update(ctx, inst); err != nil {
				s.logError(ctx, "fanout start: db update failed (status will be reset on next server boot)", "instance_id", id, "error", err)
			}
		}()
	}
	wg.Wait()
}

// DispatchStopByGroup 是整组停止的派发阶段：把"非 stopped/非 stopping"实例翻 stopping。
func (s *InstanceService) DispatchStopByGroup(ctx context.Context, userID, groupID uint) (transitioning []uint, skipped []uint, err error) {
	rows, err := s.instanceRepo.ListByGroup(ctx, userID, groupID)
	if err != nil {
		s.logError(ctx, "dispatch stop: list failed", "user_id", userID, "group_id", groupID, "error", err)
		return nil, nil, err
	}
	for _, inst := range rows {
		if inst.Status == emulator.InstanceStatusStopped || inst.Status == emulator.InstanceStatusStopping {
			skipped = append(skipped, inst.ID)
			continue
		}
		updated := inst
		updated.Status = emulator.InstanceStatusStopping
		if err := s.instanceRepo.Update(ctx, &updated); err != nil {
			s.logError(ctx, "dispatch stop: mark stopping failed", "instance_id", inst.ID, "error", err)
			return nil, nil, err
		}
		transitioning = append(transitioning, inst.ID)
	}
	s.logInfo(ctx, "group stop dispatched", "user_id", userID, "group_id", groupID, "transitioning", len(transitioning), "skipped", len(skipped))
	return transitioning, skipped, nil
}

// RunStopFanout 真正调用 runner.Stop。失败的实例落 error；成功的落 stopped + 清 serial。
func (s *InstanceService) RunStopFanout(ctx context.Context, ids []uint) {
	if len(ids) == 0 {
		return
	}
	const concurrency = 8
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					s.logError(ctx, "fanout stop panic", "instance_id", id, "panic", r)
					s.markInstanceError(ctx, id)
				}
			}()
			inst, err := s.instanceRepo.FindByID(ctx, id)
			if err != nil || inst == nil {
				s.logError(ctx, "fanout stop: instance not found", "instance_id", id, "error", err)
				return
			}
			if inst.Serial != "" {
				if err := s.runner.Stop(ctx, inst.Serial); err != nil {
					s.logError(ctx, "fanout stop: runner failed", "instance_id", id, "serial", inst.Serial, "error", err)
					s.markInstanceError(ctx, id)
					return
				}
			}
			inst.Status = emulator.InstanceStatusStopped
			inst.Serial = ""
			if err := s.instanceRepo.Update(ctx, inst); err != nil {
				s.logError(ctx, "fanout stop: db update failed", "instance_id", id, "error", err)
			}
		}()
	}
	wg.Wait()
}

// markInstanceError 把指定实例置为 error 状态，serial 清空。仅日志，不返回错误：
// 调用方都是后台 goroutine，没有合理的失败传递路径，server 重启会清零。
func (s *InstanceService) markInstanceError(ctx context.Context, id uint) {
	inst, err := s.instanceRepo.FindByID(ctx, id)
	if err != nil || inst == nil {
		s.logError(ctx, "markInstanceError: lookup failed", "instance_id", id, "error", err)
		return
	}
	inst.Status = emulator.InstanceStatusError
	inst.Serial = ""
	if err := s.instanceRepo.Update(ctx, inst); err != nil {
		s.logError(ctx, "markInstanceError: update failed", "instance_id", id, "error", err)
	}
}

// StopAndDeleteByGroup 给 DELETE /groups/:id 用：先并发停模拟器+删 AVD，再返回让 repo 清库行。
//
// 即使个别 stop / delete 报错也继续走完，避免一个失败堵住整组清理；错误统一打到日志。
func (s *InstanceService) StopAndDeleteByGroup(ctx context.Context, userID, groupID uint) error {
	rows, err := s.instanceRepo.ListByGroup(ctx, userID, groupID)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for i := range rows {
		wg.Add(1)
		go func(inst emulator.Instance) {
			defer wg.Done()
			if inst.Serial != "" {
				if err := s.runner.Stop(ctx, inst.Serial); err != nil {
					s.logError(ctx, "group cleanup stop failed", "instance", inst.Name, "error", err)
				}
			}
			if err := s.avd.Delete(ctx, inst.Name); err != nil {
				s.logError(ctx, "group cleanup avd delete failed", "instance", inst.Name, "error", err)
			}
		}(rows[i])
	}
	wg.Wait()
	return nil
}

// LookupOwnedInstance 暴露给其他应用包（如 group handler 回填新建实例字段）。
func (s *InstanceService) LookupOwnedInstance(ctx context.Context, userID, instanceID uint) (emulator.Instance, error) {
	inst, err := s.loadOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return emulator.Instance{}, err
	}
	return *inst, nil
}

// loadOwnedInstance 用来把"查实例 + 校验归属"这件事集中到一处，减少重复判断。
//
// 设计要点：
//   - 先按主键去查实例本身：查不到说明真没这条记录，返回 ErrInstanceNotFound（对外 404）。
//   - 查到后再比对 UserID：归属对不上说明是别人的实例，返回 ErrForbidden（对外 403）。
func (s *InstanceService) loadOwnedInstance(ctx context.Context, userID uint, instanceID uint) (*emulator.Instance, error) {
	instance, err := s.instanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.UserID != userID {
		return nil, ErrForbidden
	}

	return instance, nil
}

// logInfo 用来在 logger 存在时打 Info 级日志；通过 ctx 让 ContextHandler 自动补 request_id。
func (s *InstanceService) logInfo(ctx context.Context, msg string, args ...any) {
	if s.logger != nil {
		s.logger.InfoContext(ctx, msg, args...)
	}
}

// logError 用来在 logger 存在时记录底层命令错误，避免每个调用点都判空。
// 接收 ctx 是为了让 ContextHandler 把 request_id 自动注入到日志记录里。
func (s *InstanceService) logError(ctx context.Context, msg string, args ...any) {
	if s.logger != nil {
		s.logger.ErrorContext(ctx, msg, args...)
	}
}

// isValidMode 用来限制实例模式只接受当前阶段明确支持的 3 种值。
func isValidMode(mode emulator.InstanceMode) bool {
	switch mode {
	case emulator.InstanceModeReusable, emulator.InstanceModeEphemeral, emulator.InstanceModeDebug:
		return true
	default:
		return false
	}
}

// noopAVDManager 表示空操作的 AVD 管理器，给单元测试和无 Android SDK 的环境兜底。
type noopAVDManager struct{}

// Create 用来在 noop 实现下直接返回成功，让测试不必准备 avdmanager。
func (noopAVDManager) Create(_ context.Context, _ string, _ CreateAVDSpec) error { return nil }

// Delete 用来在 noop 实现下直接返回成功，让测试不必准备 avdmanager。
func (noopAVDManager) Delete(_ context.Context, _ string) error { return nil }

// noopEmulatorRunner 表示空操作的模拟器运行器，启动时返回固定的占位 serial。
type noopEmulatorRunner struct{}

// Start 用来在 noop 实现下直接返回固定 serial，方便单元测试。
func (noopEmulatorRunner) Start(_ context.Context, _ string) (string, error) {
	return "emulator-noop", nil
}

// Stop 用来在 noop 实现下直接返回成功，方便单元测试。
func (noopEmulatorRunner) Stop(_ context.Context, _ string) error { return nil }
