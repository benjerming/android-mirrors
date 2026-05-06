package httpapi

import (
	"log/slog"
	"net/http"

	appinstance "assassin-android-controller/internal/application/instance"

	"github.com/gin-gonic/gin"
)

// TemplateHandler 表示模板 HTTP 入口，负责把模板列表以轻量 JSON 形式返回给前端。
type TemplateHandler struct {
	service *appinstance.TemplateService // service 用来读取模板数据。
}

// templateResponse 表示模板列表返回给前端的最小字段集合。
type templateResponse struct {
	ID          uint   `json:"id"`          // ID 表示模板编号，前端创建实例时会回传它。
	Name        string `json:"name"`        // Name 表示模板名称。
	Description string `json:"description"` // Description 表示模板说明。
	SystemImage string `json:"systemImage"` // SystemImage 表示模板底层镜像标识。
}

// NewTemplateHandler 用来创建模板 handler，通常在路由装配时调用。
func NewTemplateHandler(service *appinstance.TemplateService) *TemplateHandler {
	return &TemplateHandler{service: service}
}

// List 用来返回当前可选模板列表，给实例新建弹窗展示下拉选项。
func (h *TemplateHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	templates, err := h.service.List(ctx)
	if err != nil {
		_ = c.Error(err)
		slog.ErrorContext(ctx, "template.list: failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list templates failed"})
		return
	}

	response := make([]templateResponse, 0, len(templates))
	for _, item := range templates {
		response = append(response, templateResponse{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			SystemImage: item.SystemImage,
		})
	}

	slog.DebugContext(ctx, "template.list: ok", "count", len(response))
	c.JSON(http.StatusOK, response)
}
