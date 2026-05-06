// Package app 提供 APK 安装 / 卸载 / 清缓存的应用层。
package app

import (
	"context"
	"errors"
	"log/slog"

	appartifact "assassin-android-controller/internal/application/artifact"
	appinstance "assassin-android-controller/internal/application/instance"
)

var (
	ErrNoSerial = errors.New("app: instance not running")
)

// ADBClient 抽象 adb.Client 的最小接口（subset of internal/infrastructure/adb.Client）。
type ADBClient interface {
	Install(ctx context.Context, serial, apkPath string) error
	Uninstall(ctx context.Context, serial, pkg string) error
	ClearAppCache(ctx context.Context, serial, pkg string) error
	SetAppLocale(ctx context.Context, serial, pkg, language string) error
}

// Service 表示 app 应用服务。
type Service struct {
	instances *appinstance.InstanceService
	artifacts *appartifact.Service
	adb       ADBClient
	logger    *slog.Logger
}

// New 构造 Service。
func New(instances *appinstance.InstanceService, artifacts *appartifact.Service, adb ADBClient, logger *slog.Logger) *Service {
	return &Service{instances: instances, artifacts: artifacts, adb: adb, logger: logger}
}

// InstallResult 表示一次 APK 安装的结果。
type InstallResult struct {
	PackageName  string `json:"packageName"`
	LocaleApplied bool  `json:"localeApplied"`
}

// Install 走 spec §4.6 的链路：归属校验 → adb install → 安装成功后异步 set-app-locales。
//
// locale 失败仅记 warning，不影响返回值。
func (s *Service) Install(ctx context.Context, userID, instanceID, artifactID uint) (InstallResult, error) {
	var res InstallResult
	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return res, err
	}
	if inst.Serial == "" {
		return res, ErrNoSerial
	}
	a, err := s.artifacts.FindByID(ctx, userID, artifactID)
	if err != nil {
		return res, err
	}
	if err := s.adb.Install(ctx, inst.Serial, a.Path); err != nil {
		return res, err
	}
	res.PackageName = a.PackageName
	if a.PackageName != "" && inst.Language != "" {
		if err := s.adb.SetAppLocale(ctx, inst.Serial, a.PackageName, inst.Language); err != nil {
			s.warn(ctx, "set-app-locales failed", "pkg", a.PackageName, "lang", inst.Language, "error", err)
		} else {
			res.LocaleApplied = true
		}
	}
	return res, nil
}

// Uninstall 卸载指定包名。
func (s *Service) Uninstall(ctx context.Context, userID, instanceID uint, pkg string) error {
	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}
	if inst.Serial == "" {
		return ErrNoSerial
	}
	return s.adb.Uninstall(ctx, inst.Serial, pkg)
}

// ClearCache 清除指定包的数据。
func (s *Service) ClearCache(ctx context.Context, userID, instanceID uint, pkg string) error {
	inst, err := s.instances.LookupOwnedInstance(ctx, userID, instanceID)
	if err != nil {
		return err
	}
	if inst.Serial == "" {
		return ErrNoSerial
	}
	return s.adb.ClearAppCache(ctx, inst.Serial, pkg)
}

func (s *Service) warn(ctx context.Context, msg string, args ...any) {
	if s.logger != nil {
		s.logger.WarnContext(ctx, msg, args...)
	}
}
