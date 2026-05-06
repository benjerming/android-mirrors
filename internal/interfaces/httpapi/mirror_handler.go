package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	appcontrol "assassin-android-controller/internal/application/control"
	appinstance "assassin-android-controller/internal/application/instance"
	appmirror "assassin-android-controller/internal/application/mirror"
	infrascrcpy "assassin-android-controller/internal/infrastructure/scrcpy"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// scrcpyFingerPointerID 与 control_service.go 同义；镜像 WS 单指注入。
const scrcpyFingerPointerID uint64 = 0xFFFFFFFFFFFFFFFE

// touchDispatcher 是 WS 读循环需要的"调用 scrcpy touch"切片，抽接口便于测试替身。
type touchDispatcher interface {
	Touch(ctx context.Context, userID, instanceID uint,
		action infrascrcpy.Action, pointerID uint64, x, y int32, pressure float32) error
}

// MirrorHandler 表示镜像接口的 HTTP 入口：升级为 WS 后把 fMP4 字节流推到客户端。
type MirrorHandler struct {
	svc          *appmirror.Service
	control      *appcontrol.Service
	dispatchOver touchDispatcher // 仅测试覆写；nil 时走 control。
}

func NewMirrorHandler(svc *appmirror.Service, control *appcontrol.Service) *MirrorHandler {
	return &MirrorHandler{svc: svc, control: control}
}

func (h *MirrorHandler) dispatcher() touchDispatcher {
	if h.dispatchOver != nil {
		return h.dispatchOver
	}
	return h.control
}

// mirrorUpgrader 用于把 HTTP 连接升级到 WebSocket。
//
// CheckOrigin 直接放行：本服务部署在内网且前置 auth 已经在中间件里完成；
// 同源策略由网关 / Cookie SameSite 把关。
var mirrorUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 64 * 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

// Stream 升级 WS → 调 mirror.Service.Stream → 把 fMP4 字节当 binary message 推出。
//
// 客户端断开 / 实例 stop / scrcpy 异常都会让 svc.Stream 返回错误；按错误类型
// 选择关闭原因，前端 transport 据此决定是否重试。
func (h *MirrorHandler) Stream(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "mirror.stream: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}

	// fmt=raw 走 WebCodecs 路径（13B header + Annex-B）；缺省/未识别均回退 fMP4，
	// 兼容老前端。前端默认请求 raw，见 web/src/lib/mirror/websocket.ts。
	format := appmirror.FormatFmp4
	if c.Query("fmt") == "raw" {
		format = appmirror.FormatRaw
	}

	conn, err := mirrorUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.WarnContext(ctx, "mirror.stream: upgrade failed",
			"user_id", user.ID, "instance_id", instanceID, "error", err.Error())
		return
	}
	defer conn.Close()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.runControlReader(streamCtx, conn, user.ID, instanceID, cancel)
	}()

	w := &wsBinaryWriter{conn: conn}
	streamErr := h.svc.Stream(streamCtx, user.ID, instanceID, 30, format, w)

	// 关闭 conn 让 reader 循环里的 ReadMessage 立即返回。
	_ = conn.Close()
	wg.Wait()

	closeCode, closeText := classifyMirrorClose(streamErr)
	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(closeCode, closeText),
		time.Now().Add(2*time.Second))

	if streamErr != nil {
		slog.WarnContext(ctx, "mirror.stream: closed",
			"user_id", user.ID, "instance_id", instanceID, "error", streamErr.Error(),
			"close_code", closeCode)
	}
}

// runControlReader 在镜像 WS 上读"客户端 → 服务端"控制帧并派发到 scrcpy。
//
// 读到任何错误（含正常关闭）就 cancel 父 ctx，让 svc.Stream 退出；同时单条
// 解析 / 派发失败不会断流，仅打 Debug 日志，避免脏数据 / 抖动一次性结束镜像。
func (h *MirrorHandler) runControlReader(
	ctx context.Context, conn *websocket.Conn,
	userID, instanceID uint, cancel context.CancelFunc,
) {
	const maxMsgsPerSecond = 200
	limiter := newSimpleRateLimiter(maxMsgsPerSecond)
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			cancel()
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		if !limiter.allow() {
			slog.WarnContext(ctx, "mirror.stream: control rate exceeded",
				"user_id", userID, "instance_id", instanceID)
			continue
		}
		frame, perr := parseMirrorControlFrame(data)
		if perr != nil {
			slog.DebugContext(ctx, "mirror.stream: bad control frame",
				"user_id", userID, "instance_id", instanceID, "error", perr.Error())
			continue
		}
		if frame.Type != "touch" {
			continue
		}
		if err := h.dispatcher().Touch(ctx, userID, instanceID,
			frame.Action, scrcpyFingerPointerID, frame.X, frame.Y, frame.Pressure); err != nil {
			slog.DebugContext(ctx, "mirror.stream: touch dispatch failed",
				"user_id", userID, "instance_id", instanceID, "error", err.Error())
		}
	}
}

// simpleRateLimiter 1 秒计数器，粗粒度防控制帧泛滥；不引入 x/time/rate 依赖。
type simpleRateLimiter struct {
	mu       sync.Mutex
	rate     int
	count    int
	windowAt time.Time
}

func newSimpleRateLimiter(rate int) *simpleRateLimiter {
	return &simpleRateLimiter{rate: rate, windowAt: time.Now()}
}

func (l *simpleRateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.windowAt) >= time.Second {
		l.windowAt = now
		l.count = 0
	}
	if l.count >= l.rate {
		return false
	}
	l.count++
	return true
}

// classifyMirrorClose 把 svc.Stream 错误映射成 ws close code，便于前端区分行为。
func classifyMirrorClose(err error) (int, string) {
	switch {
	case err == nil:
		return websocket.CloseNormalClosure, ""
	case errors.Is(err, appinstance.ErrInstanceNotFound), errors.Is(err, appinstance.ErrForbidden):
		return websocket.ClosePolicyViolation, "auth"
	case errors.Is(err, appmirror.ErrNotReady):
		return websocket.CloseTryAgainLater, "not ready"
	default:
		return websocket.CloseInternalServerErr, "internal"
	}
}

// wsBinaryWriter 把 io.Writer 写入桥接到 ws.WriteMessage(BinaryMessage)。
//
// fMP4 muxer 输出的每段（init segment / fragment）都是一次完整 Write，
// 直接对应一条 binary message，便于浏览器端按 message 喂给 SourceBuffer。
type wsBinaryWriter struct {
	conn *websocket.Conn
}

func (w *wsBinaryWriter) Write(p []byte) (int, error) {
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
