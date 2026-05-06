package httpapi

import (
	"log/slog"
	"net/http"

	appconfig "assassin-android-controller/internal/application/config"

	"github.com/gin-gonic/gin"
)

// ConfigHandler 暴露配置选项给前端，目前只有一个 Options 接口。
type ConfigHandler struct {
	svc *appconfig.Service
}

// NewConfigHandler 用来创建 ConfigHandler。
func NewConfigHandler(svc *appconfig.Service) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

// Options 返回 profiles + languages，前端启动时拉一次缓存。
func (h *ConfigHandler) Options(c *gin.Context) {
	o := h.svc.Options()
	slog.DebugContext(c.Request.Context(), "config.options: ok",
		"profiles", len(o.Profiles), "languages", len(o.Languages))
	c.JSON(http.StatusOK, gin.H{"profiles": o.Profiles, "languages": o.Languages})
}
