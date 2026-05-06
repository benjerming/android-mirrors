package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	appinstance "assassin-android-controller/internal/application/instance"
	"assassin-android-controller/internal/domain/emulator"

	"github.com/gin-gonic/gin"
)

// InstanceHandler 表示实例 HTTP 入口，负责列表、创建和启停删这些阶段 1 能力。
type InstanceHandler struct {
	service *appinstance.InstanceService // service 用来执行业务逻辑并做实例归属校验。
}

// instanceCreateRequest 表示前端新建实例时传来的最小请求体。
//
// 字段命名规则：JSON 用前端友好的小驼峰（templateId / tag / mode）。
// 早期草案里曾经存在 templateID 大写写法和 name 字段，现已统一收敛到下面这三个字段，
// 减少前后端来回沟通字段含义的成本。
type instanceCreateRequest struct {
	TemplateID uint   `json:"templateId"` // TemplateID 表示用户选中的模板编号。
	Tag        string `json:"tag"`        // Tag 表示实例标签，会用来生成实例完整名。
	Mode       string `json:"mode"`       // Mode 表示实例保留模式（reusable/ephemeral/debug）。
}

// instanceResponse 表示实例列表和创建接口返回给前端的最小数据结构。
type instanceResponse struct {
	ID         uint   `json:"id"`         // ID 表示实例编号。
	Name       string `json:"name"`       // Name 表示实例完整名称。
	Tag        string `json:"tag"`        // Tag 表示实例标签。
	Mode       string `json:"mode"`       // Mode 表示实例保留模式。
	Status     string `json:"status"`     // Status 表示实例当前运行状态。
	TemplateID uint   `json:"templateId"` // TemplateID 表示实例模板编号。
}

// NewInstanceHandler 用来创建实例 handler，通常在应用启动装配路由时调用。
func NewInstanceHandler(service *appinstance.InstanceService) *InstanceHandler {
	return &InstanceHandler{service: service}
}

// List 用来返回当前用户自己的实例列表，供实例页主列表展示。
func (h *InstanceHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	currentUser, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	items, err := h.service.ListByUser(ctx, currentUser.ID)
	if err != nil {
		_ = c.Error(err)
		slog.ErrorContext(ctx, "instance.list: failed", "user_id", currentUser.ID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list instances failed"})
		return
	}

	slog.DebugContext(ctx, "instance.list: ok", "user_id", currentUser.ID, "count", len(items))
	c.JSON(http.StatusOK, mapInstances(items))
}

// Create 用来按模板、tag 和 mode 新建实例，并立刻把结果回给前端。
func (h *InstanceHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	currentUser, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var request instanceCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		slog.WarnContext(ctx, "instance.create: invalid body", "user_id", currentUser.ID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	tag := strings.TrimSpace(request.Tag)
	mode := strings.TrimSpace(request.Mode)
	created, err := h.service.Create(ctx, currentUser.ID, appinstance.CreateInstanceInput{
		TemplateID: request.TemplateID,
		Tag:        tag,
		Mode:       emulator.InstanceMode(mode),
	})
	if err != nil {
		h.logInstanceError(ctx, "instance.create", err,
			"user_id", currentUser.ID, "template_id", request.TemplateID, "tag", tag, "mode", mode)
		h.writeInstanceError(c, err)
		return
	}

	slog.InfoContext(ctx, "instance.create: ok",
		"user_id", currentUser.ID,
		"instance_id", created.ID,
		"name", created.Name,
		"template_id", request.TemplateID,
		"tag", tag,
		"mode", mode,
	)
	c.JSON(http.StatusCreated, toInstanceResponse(*created))
}

// Start 用来启动当前用户自己的实例。
func (h *InstanceHandler) Start(c *gin.Context) {
	h.runStateMutation(c, "instance.start", h.service.Start)
}

// Stop 用来停止当前用户自己的实例。
func (h *InstanceHandler) Stop(c *gin.Context) {
	h.runStateMutation(c, "instance.stop", h.service.Stop)
}

// Delete 用来删除当前用户自己的实例。
func (h *InstanceHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	currentUser, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "instance.delete: invalid id", "user_id", currentUser.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}

	if err := h.service.Delete(ctx, currentUser.ID, instanceID); err != nil {
		h.logInstanceError(ctx, "instance.delete", err, "user_id", currentUser.ID, "instance_id", instanceID)
		h.writeInstanceError(c, err)
		return
	}

	slog.InfoContext(ctx, "instance.delete: ok", "user_id", currentUser.ID, "instance_id", instanceID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// runStateMutation 用来复用启动和停止的公共逻辑，减少重复的参数解析和权限判断。
func (h *InstanceHandler) runStateMutation(c *gin.Context, op string, action func(ctx context.Context, userID uint, instanceID uint) error) {
	ctx := c.Request.Context()
	currentUser, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	instanceID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, op+": invalid id", "user_id", currentUser.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance id"})
		return
	}

	if err := action(ctx, currentUser.ID, instanceID); err != nil {
		h.logInstanceError(ctx, op, err, "user_id", currentUser.ID, "instance_id", instanceID)
		h.writeInstanceError(c, err)
		return
	}

	slog.InfoContext(ctx, op+": ok", "user_id", currentUser.ID, "instance_id", instanceID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// logInstanceError 把业务错误按"客户端 vs 服务端"分级写日志，并保留 op + 关键字段。
//
// 服务端错误用 Error 级别（落到 5xx 监控告警），客户端错误（404/403/校验等）用 Warn。
// 已知业务错误（命名 / 模式 / 鉴权 / not found）按客户端处理，避免把正常的"参数不对"告警刷成 Error。
func (h *InstanceHandler) logInstanceError(ctx context.Context, op string, err error, kv ...any) {
	attrs := append([]any{"error", err.Error()}, kv...)
	switch {
	case errors.Is(err, appinstance.ErrInvalidTag),
		errors.Is(err, appinstance.ErrInvalidMode),
		errors.Is(err, appinstance.ErrTemplateNotFound),
		errors.Is(err, appinstance.ErrInstanceNotFound),
		errors.Is(err, appinstance.ErrForbidden):
		slog.WarnContext(ctx, op+": rejected", attrs...)
	default:
		slog.ErrorContext(ctx, op+": failed", attrs...)
	}
}

// writeInstanceError 用来把实例服务的业务错误稳定映射成 HTTP 状态码。
func (h *InstanceHandler) writeInstanceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appinstance.ErrInvalidTag), errors.Is(err, appinstance.ErrInvalidMode):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, appinstance.ErrTemplateNotFound), errors.Is(err, appinstance.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, appinstance.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "instance operation failed"})
	}
}

// parseUintParam 用来把 URL 里的实例编号解析成无符号整数，避免每个接口重复写一遍。
func parseUintParam(c *gin.Context, key string) (uint, bool) {
	value := c.Param(key)
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(parsed), true
}

// mapInstances 用来把领域里的实例列表转成前端 JSON 更容易消费的结构。
func mapInstances(items []emulator.Instance) []instanceResponse {
	response := make([]instanceResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toInstanceResponse(item))
	}
	return response
}

// toInstanceResponse 用来把单个实例领域对象转成接口响应。
func toInstanceResponse(item emulator.Instance) instanceResponse {
	return instanceResponse{
		ID:         item.ID,
		Name:       item.Name,
		Tag:        item.Tag,
		Mode:       string(item.Mode),
		Status:     string(item.Status),
		TemplateID: item.TemplateID,
	}
}
