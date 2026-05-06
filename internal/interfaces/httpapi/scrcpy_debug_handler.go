package httpapi

import (
	"net/http"

	"assassin-android-controller/internal/infrastructure/scrcpy"

	"github.com/gin-gonic/gin"
)

// ScrcpyDebugHandler 暴露 scrcpy supervisor 的运行时状态 + 人工恢复入口。
//
// 第一版只校验登录态（CurrentUser），不区分 admin。生产部署如果对外网可达，
// 应在网关或加 admin 中间件保护这些路径。
type ScrcpyDebugHandler struct {
	sm *scrcpy.SessionManager
}

func NewScrcpyDebugHandler(sm *scrcpy.SessionManager) *ScrcpyDebugHandler {
	return &ScrcpyDebugHandler{sm: sm}
}

// List GET /api/v1/debug/scrcpy/sessions —— 返回每个 supervisor 的快照。
func (h *ScrcpyDebugHandler) List(c *gin.Context) {
	if _, ok := CurrentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": h.sm.Snapshots()})
}

// Reset POST /api/v1/debug/scrcpy/sessions/:serial/reset —— 把 supervisor 的
// RestartPolicy 清零；处于 Failed 状态时这是唯一恢复入口。
func (h *ScrcpyDebugHandler) Reset(c *gin.Context) {
	if _, ok := CurrentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	serial := c.Param("serial")
	if serial == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serial required"})
		return
	}
	if err := h.sm.Reset(serial); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
