package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	appartifact "assassin-android-controller/internal/application/artifact"
	domartifact "assassin-android-controller/internal/domain/artifact"

	"github.com/gin-gonic/gin"
)

// ArtifactHandler 表示 APK 资源 HTTP 入口。
type ArtifactHandler struct {
	svc *appartifact.Service
}

func NewArtifactHandler(svc *appartifact.Service) *ArtifactHandler {
	return &ArtifactHandler{svc: svc}
}

type artifactResponse struct {
	ID          uint   `json:"id"`
	Type        string `json:"type"`
	OriginName  string `json:"originName"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	PackageName string `json:"packageName,omitempty"`
}

// Upload 接收 multipart 文件并落库。
func (h *ArtifactHandler) Upload(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		slog.WarnContext(ctx, "artifact.upload: missing form file", "user_id", user.ID, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing form file"})
		return
	}
	defer file.Close()

	a, err := h.svc.UploadAPK(ctx, appartifact.UploadAPKInput{
		UserID:     user.ID,
		OriginName: header.Filename,
		Body:       file,
	})
	if err != nil {
		if errors.Is(err, appartifact.ErrInvalidFile) {
			slog.WarnContext(ctx, "artifact.upload: invalid file",
				"user_id", user.ID, "filename", header.Filename, "size", header.Size, "error", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_ = c.Error(err)
		slog.ErrorContext(ctx, "artifact.upload: failed",
			"user_id", user.ID, "filename", header.Filename, "size", header.Size, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "artifact upload failed"})
		return
	}
	slog.InfoContext(ctx, "artifact.upload: ok",
		"user_id", user.ID,
		"artifact_id", a.ID,
		"filename", header.Filename,
		"size", a.Size,
		"sha256", a.SHA256,
		"package_name", a.PackageName,
	)
	c.JSON(http.StatusOK, toArtifactResponse(*a))
}

// List 返回当前用户全部 APK artifact。
func (h *ArtifactHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	user, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	rows, err := h.svc.List(ctx, user.ID)
	if err != nil {
		_ = c.Error(err)
		slog.ErrorContext(ctx, "artifact.list: failed", "user_id", user.ID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list artifacts failed"})
		return
	}
	out := make([]artifactResponse, len(rows))
	for i, a := range rows {
		out[i] = toArtifactResponse(a)
	}
	slog.DebugContext(ctx, "artifact.list: ok", "user_id", user.ID, "count", len(out))
	c.JSON(http.StatusOK, out)
}

func toArtifactResponse(a domartifact.Artifact) artifactResponse {
	return artifactResponse{
		ID:          a.ID,
		Type:        string(a.Type),
		OriginName:  a.OriginName,
		Size:        a.Size,
		SHA256:      a.SHA256,
		PackageName: a.PackageName,
	}
}
