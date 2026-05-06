// Package artifact 提供 APK 上传 / 列表的应用层。
package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	domartifact "assassin-android-controller/internal/domain/artifact"
	"assassin-android-controller/internal/domain/repository"
)

// ErrInvalidFile 表示上传的文件大小为 0 或文件名不合法。
var ErrInvalidFile = errors.New("artifact: invalid file")

// PackageNameParser 表示"从 apk 解析包名"的能力（aapt2.Parser 实现）。
type PackageNameParser interface {
	PackageName(ctx context.Context, apkPath string) (string, error)
}

// Service 表示 artifact 应用服务。
type Service struct {
	repo   repository.ArtifactRepository
	parser PackageNameParser
	root   string // root 表示 artifact 落地目录。
}

// New 用来构造 Service。root 必须存在或可创建，否则 Upload 会报错。
func New(repo repository.ArtifactRepository, parser PackageNameParser, root string) *Service {
	return &Service{repo: repo, parser: parser, root: root}
}

// UploadAPKInput 表示一次 APK 上传的最小参数。
type UploadAPKInput struct {
	UserID     uint
	OriginName string // OriginName 上传时的原始文件名，用于同名覆盖。
	Body       io.Reader
}

// UploadAPK 把 APK 落到磁盘 + 解析包名 + upsert 元数据。
//
// 流程：
//  1. 校验入参；
//  2. 落到 `{root}/u{uid}/{origin}` 临时路径，同时算 SHA256；
//  3. 调 parser.PackageName 拿到包名（aapt2 不可用时记 warning，包名留空）；
//  4. UpsertByOriginName 进库；同名时覆盖。
func (s *Service) UploadAPK(ctx context.Context, in UploadAPKInput) (*domartifact.Artifact, error) {
	originName := strings.TrimSpace(in.OriginName)
	if originName == "" || in.Body == nil {
		return nil, ErrInvalidFile
	}
	// 仅做最朴素的扩展名校验，避免上传 .txt 当 APK。
	if !strings.HasSuffix(strings.ToLower(originName), ".apk") {
		return nil, ErrInvalidFile
	}

	userDir := filepath.Join(s.root, fmt.Sprintf("u%d", in.UserID))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir artifact dir: %w", err)
	}
	target := filepath.Join(userDir, originName)

	f, err := os.Create(target)
	if err != nil {
		return nil, fmt.Errorf("create artifact file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, hasher), in.Body)
	if err != nil {
		return nil, fmt.Errorf("write artifact: %w", err)
	}
	if size == 0 {
		_ = os.Remove(target)
		return nil, ErrInvalidFile
	}

	pkg := ""
	if s.parser != nil {
		// 包名解析失败不阻塞上传——把 warning 留给上层日志，让记录先入库。
		if got, err := s.parser.PackageName(ctx, target); err == nil {
			pkg = got
		}
	}

	a := &domartifact.Artifact{
		UserID:      in.UserID,
		Type:        domartifact.TypeAPK,
		OriginName:  originName,
		Path:        target,
		Size:        size,
		SHA256:      hex.EncodeToString(hasher.Sum(nil)),
		PackageName: pkg,
	}
	if err := s.repo.UpsertByOriginName(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// List 当前用户全部 APK artifact。
func (s *Service) List(ctx context.Context, userID uint) ([]domartifact.Artifact, error) {
	return s.repo.List(ctx, userID, domartifact.TypeAPK)
}

// FindByID 校验归属返回 artifact，给 install 接口复用。
func (s *Service) FindByID(ctx context.Context, userID, id uint) (*domartifact.Artifact, error) {
	return s.repo.FindByID(ctx, userID, id)
}
