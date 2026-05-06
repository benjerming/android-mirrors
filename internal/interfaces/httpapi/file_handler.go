package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	appfile "assassin-android-controller/internal/application/file"
	appinstance "assassin-android-controller/internal/application/instance"

	"github.com/gin-gonic/gin"
)

// FileHandler 表示设备文件 push / 删除的 HTTP 入口（spec §3.3.5）。
type FileHandler struct {
	svc *appfile.Service
}

func NewFileHandler(svc *appfile.Service) *FileHandler {
	return &FileHandler{svc: svc}
}

func (h *FileHandler) Upload(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "file.upload: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		slog.WarnContext(ctx, "file.upload: missing form file",
			"user_id", user.ID, "instance_id", instanceID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing form file"})
		return
	}
	defer file.Close()
	remotePath := c.Request.FormValue("remotePath")
	if remotePath == "" {
		slog.WarnContext(ctx, "file.upload: missing remote path",
			"user_id", user.ID, "instance_id", instanceID, "filename", header.Filename)
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing remotePath"})
		return
	}
	if err := h.svc.Push(ctx, user.ID, instanceID, header.Filename, remotePath, file); err != nil {
		logFileError(ctx, "file.upload", err,
			"user_id", user.ID, "instance_id", instanceID,
			"filename", header.Filename, "size", header.Size, "remote_path", remotePath)
		writeFileError(c, err)
		return
	}
	slog.InfoContext(ctx, "file.upload: ok",
		"user_id", user.ID, "instance_id", instanceID,
		"filename", header.Filename, "size", header.Size, "remote_path", remotePath)
	c.JSON(http.StatusOK, gin.H{"success": true, "remotePath": remotePath})
}

type deleteFileRequest struct {
	RemotePath string `json:"remotePath"`
}

func (h *FileHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "file.delete: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}
	var req deleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RemotePath == "" {
		slog.WarnContext(ctx, "file.delete: invalid body",
			"user_id", user.ID, "instance_id", instanceID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.Delete(ctx, user.ID, instanceID, req.RemotePath); err != nil {
		logFileError(ctx, "file.delete", err,
			"user_id", user.ID, "instance_id", instanceID, "remote_path", req.RemotePath)
		writeFileError(c, err)
		return
	}
	slog.InfoContext(ctx, "file.delete: ok",
		"user_id", user.ID, "instance_id", instanceID, "remote_path", req.RemotePath)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// logFileError 把设备文件操作错误按客户端 / 服务端分级。
func logFileError(ctx context.Context, op string, err error, kv ...any) {
	attrs := append([]any{"error", err.Error()}, kv...)
	switch {
	case errors.Is(err, appinstance.ErrInstanceNotFound),
		errors.Is(err, appinstance.ErrForbidden),
		errors.Is(err, appfile.ErrNoSerial),
		errors.Is(err, appfile.ErrInvalidPath),
		errors.Is(err, appfile.ErrInvalidFile):
		slog.WarnContext(ctx, op+": rejected", attrs...)
	default:
		slog.ErrorContext(ctx, op+": failed", attrs...)
	}
}

func writeFileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appinstance.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, appinstance.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, appfile.ErrNoSerial):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, appfile.ErrInvalidPath), errors.Is(err, appfile.ErrInvalidFile):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file op failed"})
	}
}
