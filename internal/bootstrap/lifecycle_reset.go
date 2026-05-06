// Package bootstrap：见 app.go 包注释。本文件实现 server 生命周期的"无条件清零"逻辑。
package bootstrap

import (
	"context"
	"log/slog"
	"os/exec"

	"gorm.io/gorm"
)

// ResetAll 在 server 启动 / 优雅停机时调用：
//
//  1. 调 `adb kill-server`，切断 adb 守的所有 emulator 通道（qemu 子进程在 adb 断后通常会自终止）；
//  2. 把 instance_models 表里所有 status != 'stopped' 或 serial != '' 的行强制刷成 stopped + 空 serial。
//
// 这样 starting/stopping/error 三个中间态只在一次 server 进程生命周期里有意义，
// 不需要做跨重启的对账。
//
// 任一步失败都只记日志、不返回错误：调用方（NewApp / Serve）必须保持启动 / 关停链路畅通。
func ResetAll(ctx context.Context, db *gorm.DB, adbBin string, logger *slog.Logger) error {
	if adbBin != "" {
		cmd := exec.CommandContext(ctx, adbBin, "kill-server") // #nosec G204 -- 路径来自配置，可信。
		if err := cmd.Run(); err != nil && logger != nil {
			logger.Warn("lifecycle reset: adb kill-server failed (continuing)", "error", err)
		}
	}
	if db == nil {
		return nil
	}
	res := db.WithContext(ctx).
		Table("instance_models").
		Where("status <> ? OR serial <> ?", "stopped", "").
		Updates(map[string]any{"status": "stopped", "serial": ""})
	if err := res.Error; err != nil {
		if logger != nil {
			logger.Warn("lifecycle reset: db update failed", "error", err)
		}
		return nil
	}
	if logger != nil {
		logger.Info("lifecycle reset: ok", "rows_reset", res.RowsAffected, "adb_killed", adbBin != "")
	}
	return nil
}
