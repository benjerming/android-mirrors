package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	appapp "assassin-android-controller/internal/application/app"
	appinstance "assassin-android-controller/internal/application/instance"

	"github.com/gin-gonic/gin"
)

// AppHandler 表示 APK 安装 / 卸载 / 清缓存 HTTP 入口。
type AppHandler struct {
	svc *appapp.Service
}

func NewAppHandler(svc *appapp.Service) *AppHandler {
	return &AppHandler{svc: svc}
}

type installRequest struct {
	ArtifactID uint `json:"artifactId"`
}

type packageRequest struct {
	Package string `json:"package"`
}

// Install 触发 APK 安装 + 自动 locale。
func (h *AppHandler) Install(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "app.install: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}
	var req installRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ArtifactID == 0 {
		slog.WarnContext(ctx, "app.install: invalid body",
			"user_id", user.ID, "instance_id", instanceID, "artifact_id", req.ArtifactID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	res, err := h.svc.Install(ctx, user.ID, instanceID, req.ArtifactID)
	if err != nil {
		logAppError(ctx, "app.install", err,
			"user_id", user.ID, "instance_id", instanceID, "artifact_id", req.ArtifactID)
		writeAppError(c, err)
		return
	}
	slog.InfoContext(ctx, "app.install: ok",
		"user_id", user.ID, "instance_id", instanceID, "artifact_id", req.ArtifactID)
	c.JSON(http.StatusOK, res)
}

// Uninstall / ClearCache 共享 packageRequest 解析逻辑。
func (h *AppHandler) Uninstall(c *gin.Context)  { h.packageOp(c, "app.uninstall", h.svc.Uninstall) }
func (h *AppHandler) ClearCache(c *gin.Context) { h.packageOp(c, "app.clear_cache", h.svc.ClearCache) }

func (h *AppHandler) packageOp(c *gin.Context, op string, fn func(context.Context, uint, uint, string) error) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, op+": invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}
	var req packageRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Package == "" {
		slog.WarnContext(ctx, op+": invalid body", "user_id", user.ID, "instance_id", instanceID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := fn(ctx, user.ID, instanceID, req.Package); err != nil {
		logAppError(ctx, op, err,
			"user_id", user.ID, "instance_id", instanceID, "package", req.Package)
		writeAppError(c, err)
		return
	}
	slog.InfoContext(ctx, op+": ok",
		"user_id", user.ID, "instance_id", instanceID, "package", req.Package)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// logAppError 把 APK 操作的错误按客户端 / 服务端分级——已知业务错误不触发 Error 监控。
func logAppError(ctx context.Context, op string, err error, kv ...any) {
	attrs := append([]any{"error", err.Error()}, kv...)
	switch {
	case errors.Is(err, appinstance.ErrInstanceNotFound),
		errors.Is(err, appinstance.ErrForbidden),
		errors.Is(err, appapp.ErrNoSerial):
		slog.WarnContext(ctx, op+": rejected", attrs...)
	default:
		slog.ErrorContext(ctx, op+": failed", attrs...)
	}
}

func writeAppError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appinstance.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, appinstance.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, appapp.ErrNoSerial):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "app op failed"})
	}
}
