package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	appconfig "assassin-android-controller/internal/application/config"
	appgroup "assassin-android-controller/internal/application/group"
	"assassin-android-controller/internal/domain/emulator"
	domgroup "assassin-android-controller/internal/domain/group"

	"github.com/gin-gonic/gin"
)

// GroupHandler 表示分组 HTTP 入口。M3 覆盖列表 / 创建 / 改名 / 删除；M5/M7 起接入并发建机与启停。
type GroupHandler struct {
	svc            *appgroup.Service
	config         *appconfig.Service                                                                                             // config 用来回填 profileDisplayName。
	starter        appgroup.GroupStarter                                                                                          // starter 处理整组启动（M7）。
	stopper        appgroup.GroupStopper                                                                                          // stopper 处理整组停止（M7）。
	lookupInstance func(ctx context.Context, userID, instanceID uint) (emulator.Instance, bool)                                   // lookupInstance 用来回填新建实例的最小字段。
	detailLookup   func(ctx context.Context, userID, groupID uint) (domgroup.GroupStats, []emulator.Instance, error)              // detailLookup 给 GET /groups/:id 用。
	bgCtx          context.Context                                                                                                // bgCtx 是 server-级长 ctx，传给后台 fanout goroutine；HTTP 请求 ctx 不可用（会被取消）。
}

// NewGroupHandler 用来构造 GroupHandler；config 允许传 nil（profileDisplayName 会留空）。
func NewGroupHandler(svc *appgroup.Service, config *appconfig.Service) *GroupHandler {
	return &GroupHandler{svc: svc, config: config, bgCtx: context.Background()}
}

// WithBackgroundCtx 注入 server-级长 ctx，用于派发后台 fanout goroutine。
// 未注入时退化为 context.Background()。
func (h *GroupHandler) WithBackgroundCtx(ctx context.Context) *GroupHandler {
	if ctx != nil {
		h.bgCtx = ctx
	}
	return h
}

// WithGroupActions 注入整组启停 + 实例回填依赖；M3 阶段允许全部传 nil。
func (h *GroupHandler) WithGroupActions(starter appgroup.GroupStarter, stopper appgroup.GroupStopper,
	lookup func(ctx context.Context, userID, instanceID uint) (emulator.Instance, bool)) *GroupHandler {
	h.starter = starter
	h.stopper = stopper
	h.lookupInstance = lookup
	return h
}

// WithDetailLookup 注入"按分组取详情"的能力。
func (h *GroupHandler) WithDetailLookup(
	lookup func(ctx context.Context, userID, groupID uint) (domgroup.GroupStats, []emulator.Instance, error),
) *GroupHandler {
	h.detailLookup = lookup
	return h
}

// groupResponse 表示列表 / 单条接口共用的响应结构。
type groupResponse struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name"`
	ProfileID          string `json:"profileId"`
	ProfileDisplayName string `json:"profileDisplayName,omitempty"`
	InstanceCount      int    `json:"instanceCount"`
	RunningCount       int    `json:"runningCount"`
	ErrorCount         int    `json:"errorCount"`
	AggregateState     string `json:"aggregateState"`
}

type createGroupRequest struct {
	Name      string   `json:"name"`
	ProfileID string   `json:"profileId"`
	Languages []string `json:"languages"`
}

// failedItem 表示分组创建过程中失败的某个语言（B-12 接入并发建 AVD 后才会有值）。
type failedItem struct {
	Language string `json:"language"`
	Error    string `json:"error"`
}

type createGroupResponse struct {
	Group     groupResponse      `json:"group"`
	Instances []instanceResponse `json:"instances"`
	Failed    []failedItem       `json:"failed"`
}

type renameGroupRequest struct {
	Name string `json:"name"`
}

// List 用来返回当前用户全部分组，含聚合统计与状态。
func (h *GroupHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rows, err := h.svc.ListWithStats(ctx, user.ID)
	if err != nil {
		_ = c.Error(err)
		slog.ErrorContext(ctx, "group.list: failed", "user_id", user.ID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list groups failed"})
		return
	}

	out := make([]groupResponse, len(rows))
	for i, g := range rows {
		out[i] = h.toResponse(g)
	}
	slog.DebugContext(ctx, "group.list: ok", "user_id", user.ID, "count", len(out))
	c.JSON(http.StatusOK, out)
}

// Create 用来按 spec §4.2 事务1 建分组；并发建 AVD 由 B-12 接入，本阶段返回空 instances/failed。
func (h *GroupHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.WarnContext(ctx, "group.create: invalid body", "user_id", user.ID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	profileID := strings.TrimSpace(req.ProfileID)
	res, err := h.svc.Create(ctx, user.ID, appgroup.CreateInput{
		Name:      name,
		ProfileID: profileID,
		Languages: req.Languages,
	})
	if err != nil {
		logGroupError(ctx, "group.create", err,
			"user_id", user.ID, "name", name, "profile_id", profileID, "languages", req.Languages)
		writeGroupError(c, err)
		return
	}

	failed := make([]failedItem, len(res.Failed))
	for i, f := range res.Failed {
		failed[i] = failedItem{Language: f.Language, Error: f.Error}
	}
	instances := make([]instanceResponse, 0, len(res.InstanceIDs))
	if h.lookupInstance != nil {
		for _, id := range res.InstanceIDs {
			if inst, ok := h.lookupInstance(ctx, user.ID, id); ok {
				instances = append(instances, toInstanceResponse(inst))
			}
		}
	}

	stats := domgroup.GroupStats{Group: res.Group, InstanceCount: len(res.InstanceIDs), RunningCount: 0}
	logLevel := slog.LevelInfo
	if len(res.Failed) > 0 {
		logLevel = slog.LevelWarn // 部分语言建机失败：业务上半成功，运维需要看到。
	}
	slog.Log(ctx, logLevel, "group.create: ok",
		"user_id", user.ID,
		"group_id", res.Group.ID,
		"name", name,
		"profile_id", profileID,
		"requested", len(req.Languages),
		"created", len(res.InstanceIDs),
		"failed", len(res.Failed),
	)
	c.JSON(http.StatusOK, createGroupResponse{
		Group:     h.toResponse(stats),
		Instances: instances,
		Failed:    failed,
	})
}

// Rename 用来改名，遇 409 / 404 / 400 分别走不同状态码。
func (h *GroupHandler) Rename(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	groupID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "group.rename: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	var req renameGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.WarnContext(ctx, "group.rename: invalid body", "user_id", user.ID, "group_id", groupID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if err := h.svc.Rename(ctx, user.ID, groupID, name); err != nil {
		logGroupError(ctx, "group.rename", err, "user_id", user.ID, "group_id", groupID, "name", name)
		writeGroupError(c, err)
		return
	}
	slog.InfoContext(ctx, "group.rename: ok", "user_id", user.ID, "group_id", groupID, "name", name)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type groupActionResponse struct {
	Transitioning []uint              `json:"transitioning"`
	Skipped       []uint              `json:"skipped"`
	Failed        []groupActionFailed `json:"failed"`
}

type groupActionFailed struct {
	InstanceID uint   `json:"instanceId"`
	Error      string `json:"error"`
}

// Start 整组并发启动（已 running 跳过）。
func (h *GroupHandler) Start(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "group.start: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	if h.starter == nil {
		slog.WarnContext(ctx, "group.start: starter not configured", "user_id", user.ID, "group_id", groupID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "group start not configured"})
		return
	}
	transitioning, skipped, err := h.svc.StartByGroup(ctx, user.ID, groupID, h.starter)
	if err != nil {
		logGroupError(ctx, "group.start", err, "user_id", user.ID, "group_id", groupID)
		writeGroupError(c, err)
		return
	}
	go h.starter.RunStartFanout(h.bgCtx, transitioning)
	slog.InfoContext(ctx, "group.start: dispatched",
		"user_id", user.ID, "group_id", groupID,
		"transitioning", len(transitioning), "skipped", len(skipped))
	c.JSON(http.StatusAccepted, groupActionResponse{
		Transitioning: transitioning,
		Skipped:       skipped,
		Failed:        []groupActionFailed{},
	})
}

// Stop 整组并发停止（已 stopped 跳过）。
func (h *GroupHandler) Stop(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "group.stop: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	if h.stopper == nil {
		slog.WarnContext(ctx, "group.stop: stopper not configured", "user_id", user.ID, "group_id", groupID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "group stop not configured"})
		return
	}
	transitioning, skipped, err := h.svc.StopByGroup(ctx, user.ID, groupID, h.stopper)
	if err != nil {
		logGroupError(ctx, "group.stop", err, "user_id", user.ID, "group_id", groupID)
		writeGroupError(c, err)
		return
	}
	go h.stopper.RunStopFanout(h.bgCtx, transitioning)
	slog.InfoContext(ctx, "group.stop: dispatched",
		"user_id", user.ID, "group_id", groupID,
		"transitioning", len(transitioning), "skipped", len(skipped))
	c.JSON(http.StatusAccepted, groupActionResponse{
		Transitioning: transitioning,
		Skipped:       skipped,
		Failed:        []groupActionFailed{},
	})
}

// Detail 返回单个分组的详细信息（含实例列表），给镜像页一次拉到位。
//
// 当前阶段直接复用 ListWithStats 找到单条 + ListByGroup 拿实例；
// 后端 B-15 之后如果新增更细粒度状态（如 starting/stopping），handler 不必改造。
func (h *GroupHandler) Detail(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "group.detail: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	if h.detailLookup == nil {
		slog.WarnContext(ctx, "group.detail: lookup not configured", "user_id", user.ID, "group_id", groupID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "detail lookup not configured"})
		return
	}
	stats, instances, err := h.detailLookup(ctx, user.ID, groupID)
	if err != nil {
		logGroupError(ctx, "group.detail", err, "user_id", user.ID, "group_id", groupID)
		writeGroupError(c, err)
		return
	}
	out := struct {
		Group     groupResponse      `json:"group"`
		Instances []instanceResponse `json:"instances"`
	}{
		Group:     h.toResponse(stats),
		Instances: make([]instanceResponse, len(instances)),
	}
	for i, inst := range instances {
		out.Instances[i] = toInstanceResponse(inst)
	}
	c.JSON(http.StatusOK, out)
}

// Delete 用来级联删除分组（DB 层在 GroupRepoGorm.Delete 里完成）。
func (h *GroupHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	groupID, ok := parseUintParam(c, "id")
	if !ok {
		slog.WarnContext(ctx, "group.delete: invalid id", "user_id", user.ID, "raw", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	if err := h.svc.Delete(ctx, user.ID, groupID); err != nil {
		logGroupError(ctx, "group.delete", err, "user_id", user.ID, "group_id", groupID)
		writeGroupError(c, err)
		return
	}
	slog.InfoContext(ctx, "group.delete: ok", "user_id", user.ID, "group_id", groupID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// toResponse 把 GroupStats 转成对外结构，profileDisplayName 由 ConfigService 回填。
func (h *GroupHandler) toResponse(s domgroup.GroupStats) groupResponse {
	displayName := ""
	if h.config != nil {
		if p, ok := h.config.Profile(s.ProfileID); ok {
			displayName = p.DisplayName
		}
	}
	return groupResponse{
		ID:                 s.ID,
		Name:               s.Name,
		ProfileID:          s.ProfileID,
		ProfileDisplayName: displayName,
		InstanceCount:      s.InstanceCount,
		RunningCount:       s.RunningCount,
		ErrorCount:         s.ErrorCount,
		AggregateState:     s.AggregateState(),
	}
}

// logGroupError 把分组业务错误按"客户端 vs 服务端"分级写日志。
//
// 已知业务错误（重名 / 非法参数 / 找不到）按 Warn——这是预期的拒绝，运维不需要被吵醒；
// 其他未知错误用 Error，会被请求中间件再走一次 5xx 监控。
func logGroupError(ctx context.Context, op string, err error, kv ...any) {
	attrs := append([]any{"error", err.Error()}, kv...)
	switch {
	case errors.Is(err, appgroup.ErrNameTaken),
		errors.Is(err, appgroup.ErrInvalidProfile),
		errors.Is(err, appgroup.ErrInvalidLang),
		errors.Is(err, appgroup.ErrNotFound),
		errors.Is(err, domgroup.ErrInvalidName):
		slog.WarnContext(ctx, op+": rejected", attrs...)
	default:
		slog.ErrorContext(ctx, op+": failed", attrs...)
	}
}

// writeGroupError 把分组业务错误映射到 HTTP 状态码。
func writeGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appgroup.ErrNameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, appgroup.ErrInvalidProfile),
		errors.Is(err, appgroup.ErrInvalidLang),
		errors.Is(err, domgroup.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, appgroup.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "group operation failed"})
	}
}
