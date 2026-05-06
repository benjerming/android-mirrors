// Package httpapi 提供后端的 HTTP 入口（gin handler 层）。
// 故意不叫 http，是为了避免与标准库 net/http 同名而到处需要别名。
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	appuser "assassin-android-controller/internal/application/user"
	domainuser "assassin-android-controller/internal/domain/user"

	"github.com/gin-gonic/gin"
)

const currentUserContextKey = "current_user"

// SessionHandler 表示会话 HTTP 入口，负责登录、会话自检和鉴权中间件。
type SessionHandler struct {
	service *appuser.SessionService // service 用来处理真正的会话业务逻辑。
}

// sessionLoginRequest 表示登录接口接收的最小请求体。
type sessionLoginRequest struct {
	Username string `json:"username"` // Username 表示用户输入的登录名。
}

// sessionLoginResponse 表示登录接口返回给前端的最小数据结构。
type sessionLoginResponse struct {
	Token string `json:"token"` // Token 表示后续请求要放进 Authorization 头里的会话令牌。
}

// sessionProfileResponse 表示会话自检接口返回的数据结构。
type sessionProfileResponse struct {
	Username string `json:"username"` // Username 表示当前 token 对应的用户名，给顶部栏和路由守卫使用。
}

// NewSessionHandler 用来创建会话 handler，通常在应用启动装配路由时调用。
func NewSessionHandler(service *appuser.SessionService) *SessionHandler {
	return &SessionHandler{service: service}
}

// Login 用来处理用户名登录，请求成功后返回一个前端可保存的 token。
func (h *SessionHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()
	var request sessionLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		slog.WarnContext(ctx, "session.login: invalid body", "error", err.Error(), "ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	username := strings.TrimSpace(request.Username)
	token, err := h.service.Login(ctx, request.Username)
	if err != nil {
		if errors.Is(err, appuser.ErrInvalidUsername) {
			slog.WarnContext(ctx, "session.login: invalid username", "username", username, "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_ = c.Error(err)
		slog.ErrorContext(ctx, "session.login: failed", "username", username, "ip", c.ClientIP(), "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	slog.InfoContext(ctx, "session.login: ok", "username", username, "ip", c.ClientIP())
	c.JSON(http.StatusOK, sessionLoginResponse{Token: token})
}

// Me 用来返回当前登录人的最小资料，给前端判断 token 是否仍然有效。
func (h *SessionHandler) Me(c *gin.Context) {
	currentUser, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, sessionProfileResponse{Username: currentUser.Username})
}

// Heartbeat 给前端续活的端点。当前阶段 session token 仍是无状态 HMAC，
// 因此 handler 只需要校验 token（由 RequireAuth 完成）后返回成功；
// LastActiveAt 持久化与过期回收会在 B-22/B-24/B-25 落地后再补。
func (h *SessionHandler) Heartbeat(c *gin.Context) {
	if _, ok := CurrentUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RequireAuth 用来校验 Authorization 头里的 Bearer token，并把当前用户放进上下文。
func (h *SessionHandler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		// 浏览器发起 WebSocket 升级时无法自定义 Authorization 头，
		// 因此当 Bearer 头为空时回退到 ?token= query 参数。
		if token == "" {
			token = strings.TrimSpace(c.Query("token"))
		}
		currentUser, err := h.service.GetProfile(c.Request.Context(), token)
		if err != nil {
			// 不打 token 本身（敏感）；只记录是否携带 + 失败原因，方便定位过期/伪造。
			slog.WarnContext(c.Request.Context(), "session.auth: rejected",
				"path", c.Request.URL.Path,
				"ip", c.ClientIP(),
				"has_token", token != "",
				"error", err.Error(),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(currentUserContextKey, currentUser)
		c.Next()
	}
}

// CurrentUser 用来从 gin 上下文里取出当前用户，给需要归属校验的 handler 复用。
func CurrentUser(c *gin.Context) (*domainuser.User, bool) {
	value, exists := c.Get(currentUserContextKey)
	if !exists {
		return nil, false
	}

	currentUser, ok := value.(*domainuser.User)
	return currentUser, ok
}
