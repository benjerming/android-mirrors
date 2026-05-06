package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	appcontrol "assassin-android-controller/internal/application/control"
	appinstance "assassin-android-controller/internal/application/instance"
	infrascrcpy "assassin-android-controller/internal/infrastructure/scrcpy"

	"github.com/gin-gonic/gin"
)

// ControlHandler 表示镜像控制接口的 HTTP 入口（spec §3.3.3）。
type ControlHandler struct {
	svc *appcontrol.Service
}

func NewControlHandler(svc *appcontrol.Service) *ControlHandler {
	return &ControlHandler{svc: svc}
}

type tapRequest struct{ X, Y int }
type swipeRequest struct {
	X1, Y1, X2, Y2 int
	DurationMs     int `json:"durationMs"`
}
type textRequest struct{ Text string }
type keyRequest struct{ Code int }

func (h *ControlHandler) Tap(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	var req tapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.WarnContext(ctx, "control.tap: invalid body", "user_id", user.ID, "instance_id", instanceID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.Tap(ctx, user.ID, instanceID, req.X, req.Y); err != nil {
		logControlError(ctx, "control.tap", err, "user_id", user.ID, "instance_id", instanceID, "x", req.X, "y", req.Y)
		writeControlError(c, err)
		return
	}
	// tap/swipe/key 频率高，OK 走 Debug，避免淹没日志；问题排查时按需开 Debug 即可看到。
	slog.DebugContext(ctx, "control.tap: ok", "user_id", user.ID, "instance_id", instanceID, "x", req.X, "y", req.Y)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ControlHandler) Swipe(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	var req swipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.WarnContext(ctx, "control.swipe: invalid body", "user_id", user.ID, "instance_id", instanceID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.Swipe(ctx, user.ID, instanceID, req.X1, req.Y1, req.X2, req.Y2, req.DurationMs); err != nil {
		logControlError(ctx, "control.swipe", err,
			"user_id", user.ID, "instance_id", instanceID,
			"x1", req.X1, "y1", req.Y1, "x2", req.X2, "y2", req.Y2, "duration_ms", req.DurationMs)
		writeControlError(c, err)
		return
	}
	slog.DebugContext(ctx, "control.swipe: ok",
		"user_id", user.ID, "instance_id", instanceID,
		"duration_ms", req.DurationMs)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ControlHandler) Text(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	var req textRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Text == "" {
		slog.WarnContext(ctx, "control.text: invalid body", "user_id", user.ID, "instance_id", instanceID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.Text(ctx, user.ID, instanceID, req.Text); err != nil {
		// 不打印原文（可能含敏感字符），只记录长度，足够用来定位"是不是空串/超长导致的失败"。
		logControlError(ctx, "control.text", err,
			"user_id", user.ID, "instance_id", instanceID, "len", len(req.Text))
		writeControlError(c, err)
		return
	}
	slog.DebugContext(ctx, "control.text: ok", "user_id", user.ID, "instance_id", instanceID, "len", len(req.Text))
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ControlHandler) Key(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	var req keyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.WarnContext(ctx, "control.key: invalid body", "user_id", user.ID, "instance_id", instanceID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.Key(ctx, user.ID, instanceID, req.Code); err != nil {
		logControlError(ctx, "control.key", err, "user_id", user.ID, "instance_id", instanceID, "code", req.Code)
		writeControlError(c, err)
		return
	}
	slog.DebugContext(ctx, "control.key: ok", "user_id", user.ID, "instance_id", instanceID, "code", req.Code)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Home / Back 直接走预定 keycode，省得前端记住 KEYCODE_HOME=3 / KEYCODE_BACK=4。
func (h *ControlHandler) Home(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	if err := h.svc.Key(ctx, user.ID, instanceID, 3); err != nil {
		logControlError(ctx, "control.home", err, "user_id", user.ID, "instance_id", instanceID)
		writeControlError(c, err)
		return
	}
	slog.DebugContext(ctx, "control.home: ok", "user_id", user.ID, "instance_id", instanceID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ControlHandler) Back(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	if err := h.svc.Key(ctx, user.ID, instanceID, 4); err != nil {
		logControlError(ctx, "control.back", err, "user_id", user.ID, "instance_id", instanceID)
		writeControlError(c, err)
		return
	}
	slog.DebugContext(ctx, "control.back: ok", "user_id", user.ID, "instance_id", instanceID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Touch 通过 scrcpy 控制通道发送多指触摸事件。坐标必须是设备坐标。
//
// 前端拿到 video init segment 后能解出设备 width/height，按 canvas 尺寸归一化即可。
func (h *ControlHandler) Touch(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	var req touchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	act, ok := parseAction(req.Action)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}
	if err := h.svc.Touch(ctx, user.ID, instanceID, act, req.PointerID, req.X, req.Y, req.Pressure); err != nil {
		logControlError(ctx, "control.touch", err,
			"user_id", user.ID, "instance_id", instanceID,
			"action", req.Action, "x", req.X, "y", req.Y)
		writeControlError(c, err)
		return
	}
	slog.DebugContext(ctx, "control.touch: ok", "user_id", user.ID, "instance_id", instanceID, "action", req.Action)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BackOrScreenOn 触发 scrcpy BACK_OR_SCREEN_ON 消息。
func (h *ControlHandler) BackOrScreenOn(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	var req actionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认 down，让前端可以省略 body。
		req.Action = "down"
	}
	act, ok := parseAction(req.Action)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}
	if err := h.svc.BackOrScreenOn(ctx, user.ID, instanceID, act); err != nil {
		logControlError(ctx, "control.back_or_screen_on", err, "user_id", user.ID, "instance_id", instanceID)
		writeControlError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ExpandNotificationPanel 展开通知栏。
func (h *ControlHandler) ExpandNotificationPanel(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	if err := h.svc.ExpandNotificationPanel(ctx, user.ID, instanceID); err != nil {
		logControlError(ctx, "control.expand_notification", err, "user_id", user.ID, "instance_id", instanceID)
		writeControlError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CollapsePanels 收起通知栏 / 快捷开关。
func (h *ControlHandler) CollapsePanels(c *gin.Context) {
	ctx := c.Request.Context()
	user, instanceID, ok := h.requireAuthAndID(c)
	if !ok {
		return
	}
	if err := h.svc.CollapsePanels(ctx, user.ID, instanceID); err != nil {
		logControlError(ctx, "control.collapse_panels", err, "user_id", user.ID, "instance_id", instanceID)
		writeControlError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type touchRequest struct {
	Action    string  `json:"action"`
	PointerID uint64  `json:"pointerId"`
	X         int32   `json:"x"`
	Y         int32   `json:"y"`
	Pressure  float32 `json:"pressure"`
}

type actionRequest struct {
	Action string `json:"action"`
}

// parseAction 把字符串 "down"/"up"/"move" 解析成 scrcpy.Action。
func parseAction(s string) (infrascrcpy.Action, bool) {
	switch s {
	case "down", "":
		return infrascrcpy.ActionDown, true
	case "up":
		return infrascrcpy.ActionUp, true
	case "move":
		return infrascrcpy.ActionMove, true
	default:
		return 0, false
	}
}

// logControlError 把控制接口错误按客户端 / 服务端分级——避免高频 4xx 误触发告警。
func logControlError(ctx context.Context, op string, err error, kv ...any) {
	attrs := append([]any{"error", err.Error()}, kv...)
	switch {
	case errors.Is(err, appinstance.ErrInstanceNotFound),
		errors.Is(err, appinstance.ErrForbidden),
		errors.Is(err, appcontrol.ErrNoSerial):
		slog.WarnContext(ctx, op+": rejected", attrs...)
	default:
		slog.ErrorContext(ctx, op+": failed", attrs...)
	}
}

func (h *ControlHandler) requireAuthAndID(c *gin.Context) (*authedUserPlaceholder, uint, bool) {
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, 0, false
	}
	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return nil, 0, false
	}
	return &authedUserPlaceholder{ID: user.ID}, instanceID, true
}

// authedUserPlaceholder 让 control handler 不直接耦合 domain/user.User 的所有字段。
type authedUserPlaceholder struct{ ID uint }

func writeControlError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appinstance.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, appinstance.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, appcontrol.ErrNoSerial):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "control op failed"})
	}
}
