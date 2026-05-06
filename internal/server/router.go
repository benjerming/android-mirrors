// Package server 负责把所有 HTTP handler 串成一个可运行的 Gin 路由。
//
// 同时承担与"运行时框架"相关的公共能力：
//   - SetGinMode：根据 env 切换 Gin 的 Debug/Release 模式。
//   - RequestLogger：把请求日志统一接到项目的 slog logger。
//   - serveSPA：在 NoRoute 时把请求回落到 index.html，配合前端 History 路由。
package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"assassin-android-controller/internal/interfaces/httpapi"
	pkglogger "assassin-android-controller/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// requestIDHeader 是前后端约定透传的请求关联头。前端每次请求生成一个短 ID，
// 后端在缺失时兜底生成，并写回响应头，方便浏览器侧把日志和后端日志对齐。
const requestIDHeader = "X-Request-Id"

// RequestIDKey 是把请求 ID 存入 gin.Context 的键，handler 想打业务日志时可以取出来一起带上。
const RequestIDKey = "request_id"

// newRequestID 生成 8 字节(16 位 hex) 的短 ID。够区分一次调试会话内的请求，又不至于刷屏。
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}

// HandlerSet 表示路由装配时需要的一组 handler，方便启动代码集中传入依赖。
type HandlerSet struct {
	Session  *httpapi.SessionHandler  // Session 负责登录和会话自检接口。
	Template *httpapi.TemplateHandler // Template 负责模板列表接口（Deprecated）。
	Instance *httpapi.InstanceHandler // Instance 负责实例列表和实例操作接口（Deprecated 列表 / 删除）。
	Config   *httpapi.ConfigHandler   // Config 暴露 profiles + languages 选项给前端。
	Group    *httpapi.GroupHandler    // Group 负责分组 CRUD 接口（M3 起）。
	Artifact *httpapi.ArtifactHandler // Artifact 负责 APK 上传 / 列表（M10）。
	App      *httpapi.AppHandler      // App 负责 APK 安装 / 卸载 / 清缓存（M11）。
	Control  *httpapi.ControlHandler  // Control 负责镜像控制 6 个接口（M14）。
	Mirror      *httpapi.MirrorHandler      // Mirror 负责镜像抓帧入口（M14 stub）。
	File        *httpapi.FileHandler        // File 负责设备文件 push / 删除（M18）。
	ScrcpyDebug *httpapi.ScrcpyDebugHandler // ScrcpyDebug 暴露 scrcpy supervisor 运行时快照和人工恢复入口。
}

// NewRouter 用来创建 Gin 路由并注册阶段 1 需要的全部接口。
//
// 调用方应当在创建之前自行调用 SetGinMode，把日志/Recovery 输出按环境收紧。
// logger 用来给请求日志中间件使用；传 nil 表示不打印请求日志（多见于单元测试）。
func NewRouter(handlers HandlerSet, assets http.FileSystem, logger *slog.Logger) *gin.Engine {
	engine := gin.New()

	// 中间件挂载顺序很重要：先挂 RequestLogger 让它能记录后续中间件 + handler 的耗时，
	// 再挂 Recovery 兜住 panic（这样即便 handler 崩溃，请求日志依然能被写出）。
	// RequestID 必须在 RequestLogger 之前挂，否则日志里取不到 ID。
	engine.Use(RequestID())
	if logger != nil {
		engine.Use(RequestLogger(logger))
	}
	engine.Use(gin.Recovery())

	api := engine.Group("/api/v1")
	api.POST("/session/login", handlers.Session.Login)

	authorized := api.Group("/")
	authorized.Use(handlers.Session.RequireAuth())
	authorized.GET("/session/me", handlers.Session.Me)
	authorized.POST("/session/heartbeat", handlers.Session.Heartbeat)
	authorized.GET("/configs/options", handlers.Config.Options)
	authorized.GET("/templates", handlers.Template.List)
	// Deprecated（B-33）：保留 GET 列表 / DELETE 单实例供运维浏览，移除 POST /instances；
	// 单实例启停继续保留，给镜像页副屏的 ▶/⏹ icon 使用。
	authorized.GET("/instances", handlers.Instance.List)
	authorized.POST("/instances/:id/start", handlers.Instance.Start)
	authorized.POST("/instances/:id/stop", handlers.Instance.Stop)
	authorized.DELETE("/instances/:id", handlers.Instance.Delete)
	if handlers.Group != nil {
		authorized.GET("/groups", handlers.Group.List)
		authorized.GET("/groups/:id", handlers.Group.Detail)
		authorized.POST("/groups", handlers.Group.Create)
		authorized.PATCH("/groups/:id", handlers.Group.Rename)
		authorized.DELETE("/groups/:id", handlers.Group.Delete)
		authorized.POST("/groups/:id/start", handlers.Group.Start)
		authorized.POST("/groups/:id/stop", handlers.Group.Stop)
	}
	if handlers.Artifact != nil {
		authorized.GET("/artifacts", handlers.Artifact.List)
		authorized.POST("/artifacts/upload", handlers.Artifact.Upload)
	}
	if handlers.App != nil {
		authorized.POST("/instances/:id/apps/install", handlers.App.Install)
		authorized.POST("/instances/:id/apps/uninstall", handlers.App.Uninstall)
		authorized.POST("/instances/:id/apps/clear-cache", handlers.App.ClearCache)
	}
	if handlers.Control != nil {
		authorized.POST("/instances/:id/control/tap", handlers.Control.Tap)
		authorized.POST("/instances/:id/control/swipe", handlers.Control.Swipe)
		authorized.POST("/instances/:id/control/text", handlers.Control.Text)
		authorized.POST("/instances/:id/control/key", handlers.Control.Key)
		authorized.POST("/instances/:id/control/home", handlers.Control.Home)
		authorized.POST("/instances/:id/control/back", handlers.Control.Back)
		// scrcpy 控制通道（多指 / 系统键 / 通知栏）。
		authorized.POST("/instances/:id/control/touch", handlers.Control.Touch)
		authorized.POST("/instances/:id/control/back-or-screen-on", handlers.Control.BackOrScreenOn)
		authorized.POST("/instances/:id/control/expand-notification", handlers.Control.ExpandNotificationPanel)
		authorized.POST("/instances/:id/control/collapse-panels", handlers.Control.CollapsePanels)
	}
	if handlers.Mirror != nil {
		authorized.GET("/instances/:id/mirror/ws", handlers.Mirror.Stream)
	}
	if handlers.File != nil {
		authorized.POST("/instances/:id/files/upload", handlers.File.Upload)
		authorized.DELETE("/instances/:id/files", handlers.File.Delete)
	}
	if handlers.ScrcpyDebug != nil {
		authorized.GET("/debug/scrcpy/sessions", handlers.ScrcpyDebug.List)
		authorized.POST("/debug/scrcpy/sessions/:serial/reset", handlers.ScrcpyDebug.Reset)
	}
	// /metrics 直接放在 engine 根，不走 RequireAuth — 让 prometheus scraper 简单。
	// 网关层应限制访问；如需登录，把它移进 authorized.GET 即可。
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	serveSPA(engine, assets)

	return engine
}

// SetGinMode 用来根据应用环境切换 Gin 的运行模式。
// dev 环境保持 DebugMode 方便看到 handler 警告；其他环境一律切到 ReleaseMode，
// 这样 Gin 不会再往 stderr 打印调试横幅，也能让默认的安全检查更严格。
func SetGinMode(env string) {
	if strings.EqualFold(strings.TrimSpace(env), "dev") {
		gin.SetMode(gin.DebugMode)
		return
	}
	gin.SetMode(gin.ReleaseMode)
}

// RequestID 中间件读取/生成请求关联 ID，写回响应头，并同时挂到 gin.Context 与
// 底层 http.Request 的 context 上。前者方便中间件按 key 读取，后者让 service 层
// 通过 ctx 透传时也能拿到——配合 pkglogger.ContextHandler，业务日志会自动带 id。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		c.Set(RequestIDKey, id)
		c.Writer.Header().Set(requestIDHeader, id)
		c.Request = c.Request.WithContext(pkglogger.WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}

// RequestLogger 用来用 slog 输出统一格式的请求日志，替代 Gin 自带的 stderr 文本日志。
//
// 选择把请求日志做成中间件，是因为：
//   - 业务 handler 已经熟悉 gin.Context，这里直接读 status/path 等字段更直观。
//   - 出错时可以从 c.Errors 读到 handler 主动登记的错误信息一起打出来。
//
// 字段尽量贴近社区主流访问日志（nginx combined / Envoy access log）：
// method/path/route/status/latency/bytes_in/bytes_out/ip/user_agent/referer。
// route 用 gin 路由模板（如 /api/v1/instances/:id），方便按接口聚合统计；
// 高频低价值的心跳接口降到 Debug，避免日志被刷屏。
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		status := c.Writer.Status()
		route := c.FullPath()
		latency := time.Since(start)

		attrs := []any{
			"request_id", c.GetString(RequestIDKey),
			"status", status,
			"method", c.Request.Method,
			"path", path,
			"route", route,
			"query", query,
			"ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"referer", c.Request.Referer(),
			"bytes_in", c.Request.ContentLength,
			"bytes_out", c.Writer.Size(),
			"latency_ms", latency.Milliseconds(),
			"latency", latency,
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		// 5xx → Error（运维报警）；4xx → Warn（客户端问题，便于排错）；
		// 心跳接口 2xx 降到 Debug，避免每秒一条；其余 Info。
		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("http", attrs...)
		case status >= http.StatusBadRequest:
			logger.Warn("http", attrs...)
		case route == "/api/v1/session/heartbeat":
			logger.Debug("http", attrs...)
		default:
			logger.Info("http", attrs...)
		}
	}
}

// spaFS 包装 http.FileSystem，文件不存在时回退到 /index.html。
type spaFS struct {
	fs http.FileSystem
}

// Open 在请求文件不存在时回落到 index.html，让前端路由（History 模式）继续生效。
func (s *spaFS) Open(name string) (http.File, error) {
	f, err := s.fs.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return s.fs.Open("/index.html")
	}
	return f, err
}

// serveSPA 把 dist 目录作为 SPA 静态资源挂载到 Gin。
// 请求文件存在则直接返回，不存在则返回 index.html 让前端路由接管。
//
// 由于二进制始终带着 cmd/server/dist 的 embed.FS（哪怕里面只有 .gitkeep 占位），
// 这里通过"是否能打开 index.html"判断 dist 里有没有真实前端产物：
//   - 有：当作生产模式，挂上 NoRoute 处理 SPA。
//   - 没有：当作开发模式，前端由 Vite 提供，直接跳过注册。
//
// assets 为 nil 也跳过，主要给单元测试用。
func serveSPA(engine *gin.Engine, assets http.FileSystem) {
	if assets == nil || !hasIndexHTML(assets) {
		return
	}
	fs := &spaFS{fs: assets}
	engine.NoRoute(func(c *gin.Context) {
		// 带 hash 的静态资源可以用 immutable 缓存策略，让浏览器一年内不再回源；
		// HTML 等入口文件必须 no-cache，否则前端发版后旧用户拿不到新版本。
		if isHashedAsset(c.Request.URL.Path) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		gin.WrapH(http.FileServer(fs))(c)
	})
}

// isHashedAsset 判断给定路径是不是 Vite 打包出来的带 hash 的静态资源。
func isHashedAsset(path string) bool {
	return strings.HasPrefix(path, "/assets/")
}

// hasIndexHTML 用来探测给定的文件系统里是否真正包含前端入口 index.html。
//
// 只有真正有 index.html 时，才把 SPA NoRoute 注册起来；否则说明这是 dev 模式下
// 由 Vite 服务前端，dist 里只有 .gitkeep 占位，跳过注册避免 NoRoute 误吞 API 请求。
func hasIndexHTML(assets http.FileSystem) bool {
	f, err := assets.Open("/index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
