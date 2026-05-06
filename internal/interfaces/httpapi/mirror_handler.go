package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	appinstance "assassin-android-controller/internal/application/instance"
	appmirror "assassin-android-controller/internal/application/mirror"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// MirrorHandler 表示镜像接口的 HTTP 入口：升级为 WS 后把 fMP4 字节流推到客户端。
type MirrorHandler struct {
	svc *appmirror.Service
}

func NewMirrorHandler(svc *appmirror.Service) *MirrorHandler {
	return &MirrorHandler{svc: svc}
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

	w := &wsBinaryWriter{conn: conn}
	streamErr := h.svc.Stream(ctx, user.ID, instanceID, 30, format, w)

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
