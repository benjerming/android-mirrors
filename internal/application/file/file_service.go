// Package file 提供"推送文件 / 删除文件"的应用层。
//
// spec §3.3.5：只允许操作白名单目录；前端可写但后端必须做最终兜底，
// 防止恶意路径（包含 ".." 或绝对路径越权）。
package file

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	appinstance "assassin-android-controller/internal/application/instance"
)

var (
	ErrNoSerial    = errors.New("file: instance not running")
	ErrInvalidPath = errors.New("file: path not in allowed directory")
	ErrInvalidFile = errors.New("file: invalid file")
)

// ADBFile 抽象 adb push / shell rm 能力。
type ADBFile interface {
	Push(ctx context.Context, serial, localPath, remotePath string) error
	RemoveRemote(ctx context.Context, serial, remotePath string) error
}

// Service 表示文件应用服务。
type Service struct {
	instances        *appinstance.InstanceService
	adb              ADBFile
	stagingRoot      string
	allowedDeviceDir string
}

// New 构造 Service。stagingRoot 用于把 multipart 上传文件落到本地，再交给 adb push。
func New(instances *appinstance.InstanceService, adb ADBFile, stagingRoot, allowedDeviceDir string) *Service {
	return &Service{
		instances:        instances,
		adb:              adb,
		stagingRoot:      stagingRoot,
		allowedDeviceDir: strings.TrimSpace(allowedDeviceDir),
	}
}

// Push 把 body 落到本地暂存路径，再通过 adb push 推到设备。
func (s *Service) Push(ctx context.Context, userID, instanceID uint, originName, remotePath string, body io.Reader) error {
	if !s.isAllowed(remotePath) {
		return ErrInvalidPath
	}
	originName = strings.TrimSpace(originName)
	if originName == "" || body == nil {
		return ErrInvalidFile
	}

	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}
	if inst.Serial == "" {
		return ErrNoSerial
	}

	// 落到 stagingRoot/u{uid}/i{iid}/origin。
	dir := filepath.Join(s.stagingRoot, "files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpPath := filepath.Join(dir, originName)
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, body); err != nil {
		out.Close()
		return err
	}
	out.Close()
	defer os.Remove(tmpPath)

	unlock := s.instances.LockInstance(instanceID)
	defer unlock()
	return s.adb.Push(ctx, inst.Serial, tmpPath, remotePath)
}

// Delete 删除设备上指定路径（白名单兜底校验）。
func (s *Service) Delete(ctx context.Context, userID, instanceID uint, remotePath string) error {
	if !s.isAllowed(remotePath) {
		return ErrInvalidPath
	}
	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}
	if inst.Serial == "" {
		return ErrNoSerial
	}
	unlock := s.instances.LockInstance(instanceID)
	defer unlock()
	return s.adb.RemoveRemote(ctx, inst.Serial, remotePath)
}

// isAllowed 严格前缀匹配 + 拒绝 ".." 出现，避免逃逸白名单。
func (s *Service) isAllowed(remotePath string) bool {
	if s.allowedDeviceDir == "" {
		return false
	}
	clean := strings.TrimSpace(remotePath)
	if clean == "" || strings.Contains(clean, "..") {
		return false
	}
	if !strings.HasPrefix(clean, s.allowedDeviceDir) {
		return false
	}
	return true
}
