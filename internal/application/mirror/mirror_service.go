// Package mirror 提供镜像（H.264 抓帧 → fMP4）的应用层。
package mirror

import (
	"context"
	"errors"
	"io"

	appinstance "assassin-android-controller/internal/application/instance"
)

// ErrNotReady 表示镜像底层尚未就绪（serial 为空 / 未绑定真实模拟器）。
var ErrNotReady = errors.New("mirror: not ready")

// VideoAttacher 抽象底层 video pump。SessionManager 实现该接口。
type VideoAttacher interface {
	AttachVideo(ctx context.Context, serial string, out io.Writer) error
	AttachVideoRaw(ctx context.Context, serial string, out io.Writer) error
}

// Format 是 Stream 的传输编码选择。
type Format int

const (
	// FormatFmp4 是历史路径：fMP4 init segment + fragments，给浏览器 MSE 用。
	FormatFmp4 Format = iota
	// FormatRaw 是新路径：13B header + Annex-B，给浏览器 WebCodecs 用。
	// 群控下 N 路镜像不会因 MSE 离屏节流 / live edge 漂移而冻结。
	FormatRaw
)

// Service 表示镜像应用服务。
type Service struct {
	instances *appinstance.InstanceService
	attacher  VideoAttacher
}

// New 构造 Service。attacher 为 nil 时，所有 Stream 调用返回 ErrNotReady。
func New(instances *appinstance.InstanceService, attacher VideoAttacher) *Service {
	return &Service{instances: instances, attacher: attacher}
}

// Stream 校验归属 → 取 serial → 按 fmt 委派给 attacher。
//
// fps 字段保留以兼容现有 handler 签名；真实 fps 由 SessionManager 启动参数控制。
func (s *Service) Stream(ctx context.Context, userID, instanceID uint, _ int, fmt Format, out io.Writer) error {
	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}
	if inst.Serial == "" {
		return ErrNotReady
	}
	if s.attacher == nil {
		return ErrNotReady
	}
	if fmt == FormatRaw {
		return s.attacher.AttachVideoRaw(ctx, inst.Serial, out)
	}
	return s.attacher.AttachVideo(ctx, inst.Serial, out)
}
