// Package bootstrap 负责"装配"——把配置、日志、数据库、仓储、应用服务、HTTP 路由
// 像积木一样组合成一个可运行的 App，并提供启动 / 优雅停机入口。
//
// 选择把装配收拢到这里，是为了让 cmd/server 入口尽量薄，
// 同时让集成测试可以通过 NewApp 复用同一套装配链路，避免与 main 各搭一份。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"time"

	appapp "assassin-android-controller/internal/application/app"
	appartifact "assassin-android-controller/internal/application/artifact"
	appconfig "assassin-android-controller/internal/application/config"
	appcontrol "assassin-android-controller/internal/application/control"
	appfile "assassin-android-controller/internal/application/file"
	appgroup "assassin-android-controller/internal/application/group"
	appinstance "assassin-android-controller/internal/application/instance"
	appmirror "assassin-android-controller/internal/application/mirror"
	appuser "assassin-android-controller/internal/application/user"
	"assassin-android-controller/internal/config"
	"assassin-android-controller/internal/domain/emulator"
	domgroup "assassin-android-controller/internal/domain/group"
	infraaapt2 "assassin-android-controller/internal/infrastructure/aapt2"
	infraadb "assassin-android-controller/internal/infrastructure/adb"
	infraavd "assassin-android-controller/internal/infrastructure/avd"
	infraemulator "assassin-android-controller/internal/infrastructure/emulator"
	infrarepo "assassin-android-controller/internal/infrastructure/repository"
	infrascrcpy "assassin-android-controller/internal/infrastructure/scrcpy"
	"assassin-android-controller/internal/interfaces/httpapi"
	"assassin-android-controller/internal/server"
	"assassin-android-controller/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// App 表示已经装配完成的后端应用，把配置、日志、数据库和路由放在一起管理。
//
// 字段保持公开是为了方便外部（比如 main 函数和集成测试）直接访问已经装配好的依赖；
// 真正的运行入口是 Run / Serve，调用方一般不需要单独操作这些字段。
type App struct {
	Config         *config.Config              // Config 表示应用当前运行配置，端口和数据库路径都来自这里。
	Logger         *slog.Logger                // Logger 表示全局日志实例，方便后续扩展更多运行日志。
	DB             *gorm.DB                    // DB 表示当前应用使用的数据库连接。
	Router         *gin.Engine                 // Router 表示已经注册好接口的 Gin 引擎。
	ScrcpySessions *infrascrcpy.SessionManager // ScrcpySessions 管理所有 scrcpy 镜像 + 控制会话；进程退出时 Shutdown。
}

// shutdowner 表示支持优雅停止的服务对象，方便把停机逻辑单独复用和测试。
type shutdowner interface {
	Shutdown(ctx context.Context) error
}

// NewApp 用来完成阶段 1 后端的依赖装配，包括配置、数据库、仓储、服务和路由。
//
// 参数说明：
//   - ctx：装配过程中如果需要访问数据库（比如灌入默认模板），会用这个 ctx 控制超时与取消。
//   - configPath：YAML 配置文件路径。
//   - assets：dev 模式下传 nil（前端由 Vite 提供）；prod 模式下传内嵌的 dist 文件系统。
func NewApp(ctx context.Context, configPath string, assets http.FileSystem) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logg, err := logger.New(cfg.App.Env)
	if err != nil {
		return nil, err
	}
	// 把装配出来的 logger 设为 slog 全局默认，handler 层直接 slog.InfoContext(ctx, ...)
	// 即可写日志，不必额外注入依赖；ContextHandler 会自动从 ctx 取 request_id。
	slog.SetDefault(logg)

	// 在创建 Gin 路由之前先决定全局模式：dev 留 DebugMode，方便看路由警告；其他环境一律 Release。
	server.SetGinMode(cfg.App.Env)

	db, err := gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{
		// gorm 默认会往 stdout 打印日志；这里改成 Warn 级别，让真正的慢查询/错误能被注意到，
		// 同时不会把每条 SQL 都刷到终端。后续可以再桥到 slog，目前阶段 1 不必过度抽象。
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := infrarepo.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	// 生命周期清零：把上次进程残留的中间态实例（starting/stopping/error/running）
	// 全部刷成 stopped，并 kill 已存在的 adb-server，避免下面的服务读到陈旧状态。
	_ = ResetAll(ctx, db, cfg.Android.AdbPath, logg)

	userRepo := infrarepo.NewUserRepositoryGORM(db)
	templateRepo := infrarepo.NewTemplateRepositoryGORM(db)
	instanceRepo := infrarepo.NewInstanceRepositoryGORM(db)
	groupRepo := infrarepo.NewGroupRepository(db)
	artifactRepo := infrarepo.NewArtifactRepository(db)

	if err := seedTemplates(ctx, templateRepo, cfg); err != nil {
		return nil, fmt.Errorf("seed templates: %w", err)
	}

	sessionService, err := appuser.NewSessionService(userRepo, cfg.Session.Secret)
	if err != nil {
		return nil, fmt.Errorf("new session service: %w", err)
	}
	templateService := appinstance.NewTemplateService(templateRepo)
	instanceService := appinstance.NewInstanceService(instanceRepo, templateRepo).
		WithRuntime(avdAdapter{inner: infraavd.NewManager()}, infraemulator.NewRunner().WithLogger(logg).WithLogDir("data/logs/emulator"), logg)
	// configService 还没构造（在下面），延迟到那里再 WithProfiles。

	configService, err := appconfig.New(cfg.TemplatesFile)
	if err != nil {
		return nil, fmt.Errorf("load config options: %w", err)
	}

	instanceService.WithProfiles(configService)

	groupService := appgroup.NewService(groupRepo, configService, instanceService).WithCleaner(instanceService)

	groupActions := groupActionsAdapter{inner: instanceService}
	groupHandler := httpapi.NewGroupHandler(groupService, configService).
		WithGroupActions(groupActions, groupActions, groupActions.Lookup).
		WithBackgroundCtx(ctx).
		WithDetailLookup(func(ctx context.Context, userID, groupID uint) (domgroup.GroupStats, []emulator.Instance, error) {
			rows, err := groupRepo.ListWithStats(ctx, userID)
			if err != nil {
				return domgroup.GroupStats{}, nil, err
			}
			for _, s := range rows {
				if s.ID == groupID {
					insts, err := instanceRepo.ListByGroup(ctx, userID, groupID)
					if err != nil {
						return domgroup.GroupStats{}, nil, err
					}
					return s, insts, nil
				}
			}
			return domgroup.GroupStats{}, nil, appgroup.ErrNotFound
		})

	aapt2Parser := infraaapt2.New(cfg.Android.Aapt2Path)
	// 配置未显式给出 adb 路径时，回退到 PATH 上找到的 adb，避免 dev 环境
	// 因为忘填路径就让 scrcpy session 永远报 "adb binary not configured"。
	adbPath := cfg.Android.AdbPath
	if adbPath == "" {
		if found, err := exec.LookPath("adb"); err == nil {
			adbPath = found
			logg.Info("bootstrap: adb_path not set, using PATH lookup", "adb", adbPath)
		}
	}
	adbClient := infraadb.New(adbPath)
	scrcpySessions := infrascrcpy.NewSessionManager(infrascrcpy.SessionManagerOpts{
		Adb:      infrascrcpy.NewAdbTunnel(adbPath),
		JarBytes: infrascrcpy.EmbeddedServerJar,
		Version:  infrascrcpy.ServerVersion,
		// 群控镜像针对监控场景调小：15fps 足够观察，720 长边把 1080p 缩到 720x1280
		// (横屏 1280x720)，4 路解码总开销大约只有原来的 1/4；2Mbps 在 720 下画质很干净。
		MaxFps:  10,
		MaxSize: 720,
		Bitrate: 1_000_000,
	})
	if err := infrascrcpy.RegisterMetrics(prometheus.DefaultRegisterer); err != nil {
		return nil, fmt.Errorf("register scrcpy metrics: %w", err)
	}

	artifactRoot := cfg.Paths.ArtifactRoot
	if artifactRoot == "" {
		artifactRoot = "data/artifacts"
	}
	artifactService := appartifact.New(artifactRepo, aapt2Parser, artifactRoot)
	appService := appapp.New(instanceService, artifactService, adbClient, logg)
	controlService := appcontrol.New(instanceService, adbClient).WithScrcpy(scrcpySessions)
	mirrorService := appmirror.New(instanceService, scrcpySessions)

	stagingRoot := cfg.Paths.ArtifactRoot
	if stagingRoot == "" {
		stagingRoot = "data/files"
	}
	allowedDeviceDir := cfg.Paths.AllowedDeviceDir
	if allowedDeviceDir == "" {
		allowedDeviceDir = "/sdcard/Download/"
	}
	fileService := appfile.New(instanceService, adbClient, stagingRoot, allowedDeviceDir)

	router := server.NewRouter(server.HandlerSet{
		Session:     httpapi.NewSessionHandler(sessionService),
		Template:    httpapi.NewTemplateHandler(templateService),
		Instance:    httpapi.NewInstanceHandler(instanceService),
		Config:      httpapi.NewConfigHandler(configService),
		Group:       groupHandler,
		Artifact:    httpapi.NewArtifactHandler(artifactService),
		App:         httpapi.NewAppHandler(appService),
		Control:     httpapi.NewControlHandler(controlService),
		Mirror:      httpapi.NewMirrorHandler(mirrorService, controlService),
		File:        httpapi.NewFileHandler(fileService),
		ScrcpyDebug: httpapi.NewScrcpyDebugHandler(scrcpySessions),
	}, assets, logg)

	return &App{
		Config:         cfg,
		Logger:         logg,
		DB:             db,
		Router:         router,
		ScrcpySessions: scrcpySessions,
	}, nil
}

const (
	httpReadHeaderTimeout = 5 * time.Second  // ReadHeaderTimeout 兜底，避免慢请求头攻击。
	httpReadTimeout       = 15 * time.Second // ReadTimeout 限制整次请求体读取时间。
	httpWriteTimeout      = 15 * time.Second // WriteTimeout 限制响应写入时间，防止慢客户端拖住连接。
	httpIdleTimeout       = 60 * time.Second // IdleTimeout 控制 Keep-Alive 连接的空闲时长。
	httpShutdownTimeout   = 5 * time.Second  // 优雅停机给现有请求留出的最长完成时间。
)

// NewHTTPServer 用来创建标准库 HTTP 服务，把超时和处理入口统一放在这里。
func (a *App) NewHTTPServer() *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", a.Config.App.HTTPPort),
		Handler:           a.Router,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}

// Run 用来监听配置里的端口，并在收到退出信号时优雅关闭 HTTP 服务和数据库连接。
func (a *App) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.Config.App.HTTPPort))
	if err != nil {
		return fmt.Errorf("listen http: %w", err)
	}

	return a.Serve(ctx, listener)
}

// Serve 用来在已经准备好的监听器上启动服务，方便测试和外部托管场景复用。
//
// 退出阶段会做两件事：
//  1. 通过 watchShutdown 给 HTTP 服务一段优雅停机时间。
//  2. 关掉底层数据库连接，避免文件锁残留。
func (a *App) Serve(ctx context.Context, listener net.Listener) error {
	server := a.NewHTTPServer()
	stopShutdown := watchShutdown(ctx, server, a.Logger)
	defer stopShutdown()
	// defer 是 LIFO：closeDB 最后跑（DB 句柄最后关），closeScrcpy 倒数第二，
	// resetAllOnShutdown 最先跑（这时 DB 还在，能写入清零更新）。
	defer a.closeDB()
	defer a.closeScrcpySessions()
	defer a.resetAllOnShutdown()

	err := server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}

	return nil
}

// closeScrcpySessions 在退出时关闭所有 scrcpy 会话，确保设备侧 reverse 映射
// 与 server 进程都被清理；幂等。
func (a *App) closeScrcpySessions() {
	if a.ScrcpySessions != nil {
		a.ScrcpySessions.Shutdown()
	}
}

// resetAllOnShutdown 在 server 关停时再做一次"adb kill-server + 全部实例置 stopped"，
// 保证下一个 server 进程启动前 DB 与设备态对齐。失败仅日志，不阻塞退出。
func (a *App) resetAllOnShutdown() {
	if a.DB == nil || a.Config == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ResetAll(ctx, a.DB, a.Config.Android.AdbPath, a.Logger)
}

// closeDB 用来在退出时关闭 gorm 背后的 *sql.DB；
// gorm 自身没有 Close 方法，所以必须先取出 sql.DB 再关。
func (a *App) closeDB() {
	if a.DB == nil {
		return
	}
	sqlDB, err := a.DB.DB()
	if err != nil {
		if a.Logger != nil {
			a.Logger.Warn("get underlying sql.DB", "error", err)
		}
		return
	}
	if err := sqlDB.Close(); err != nil && a.Logger != nil {
		a.Logger.Warn("close sql.DB", "error", err)
	}
}

// watchShutdown 用来监听退出信号，收到后给 HTTP 服务一个有限时间做收尾。
//
// 返回的函数用来主动取消监听协程，调用方在 Serve 返回时会用 defer 调用它，
// 防止协程泄漏。
func watchShutdown(ctx context.Context, server shutdowner, logger *slog.Logger) func() {
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
			defer cancel()

			if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) && logger != nil {
				logger.Warn("shutdown http server", "error", err)
			}
		case <-stop:
			return
		}
	}()

	return func() {
		close(stop)
	}
}

// groupActionsAdapter 把 InstanceService 的 *GroupActionFailure 翻译成 appgroup.GroupFailure，
// 让两个领域包不必相互依赖。同时提供 Lookup 给 handler 回填新建实例的最小字段。
type groupActionsAdapter struct {
	inner *appinstance.InstanceService
}

func (a groupActionsAdapter) DispatchStartByGroup(ctx context.Context, userID, groupID uint) ([]uint, []uint, error) {
	return a.inner.DispatchStartByGroup(ctx, userID, groupID)
}

func (a groupActionsAdapter) RunStartFanout(ctx context.Context, ids []uint) {
	a.inner.RunStartFanout(ctx, ids)
}

func (a groupActionsAdapter) DispatchStopByGroup(ctx context.Context, userID, groupID uint) ([]uint, []uint, error) {
	return a.inner.DispatchStopByGroup(ctx, userID, groupID)
}

func (a groupActionsAdapter) RunStopFanout(ctx context.Context, ids []uint) {
	a.inner.RunStopFanout(ctx, ids)
}

func (a groupActionsAdapter) Lookup(ctx context.Context, userID, instanceID uint) (emulator.Instance, bool) {
	inst, err := a.inner.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return emulator.Instance{}, false
	}
	return inst, true
}

// avdAdapter 用来把 infrastructure/avd.Manager 适配到 application/instance.AVDManager 接口。
//
// 两个层各自定义自己的 Spec 结构，避免应用层依赖基础设施层；这里做一次简单的字段拷贝即可。
type avdAdapter struct {
	inner *infraavd.Manager // inner 表示真正干活的底层 AVD 管理器。
}

// Create 用来把应用层的 CreateAVDSpec 翻译成基础设施层的 avd.Spec，再调用真实命令。
func (a avdAdapter) Create(ctx context.Context, instanceName string, spec appinstance.CreateAVDSpec) error {
	return a.inner.Create(ctx, instanceName, infraavd.Spec{
		SystemImage: spec.SystemImage,
		Device:      spec.Device,
		Resolution:  spec.Resolution,
		Density:     spec.Density,
	})
}

// Delete 用来直接转发到底层 AVD 管理器的删除命令。
func (a avdAdapter) Delete(ctx context.Context, instanceName string) error {
	return a.inner.Delete(ctx, instanceName)
}

// seedTemplates 用来在启动时把配置里声明的模板（含 config.json 的 avdProfiles）幂等同步进数据库，
// 让前端建机弹窗永远拿到最新的可选模板。
func seedTemplates(ctx context.Context, templateRepo *infrarepo.TemplateRepositoryGORM, cfg *config.Config) error {
	for _, item := range cfg.Templates {
		if err := templateRepo.UpsertByName(ctx, &emulator.Template{
			Name:        item.Name,
			Description: item.Description,
			SystemImage: item.SystemImage,
			Device:      item.Device,
			Resolution:  item.Resolution,
			Density:     item.Density,
		}); err != nil {
			return err
		}
	}

	return nil
}
