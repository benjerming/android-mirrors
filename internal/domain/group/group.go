// Package group 表示用户的分组——一组同模板、不同语言的模拟器。
package group

import (
	"errors"
	"regexp"
	"time"
	"unicode/utf8"
)

// ErrInvalidName 表示分组名不满足长度或字符约束。
var ErrInvalidName = errors.New("group: invalid name")

// nameRe 定义分组名允许的字符集：中文、字母、数字、下划线、中划线。
var nameRe = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_\-]+$`)

// Group 表示一个分组实体。
type Group struct {
	ID        uint
	UserID    uint
	Name      string
	ProfileID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GroupStats 表示一行分组连同其下属实例的聚合统计，给列表页一次性返回。
//
// 各计数从 emulator_instances JOIN 而来；当前阶段实例只有 running / stopped 两个状态，
// 但保留 TransitCount / ErrorCount 字段，等 B-15 起接入更多状态时直接生效。
type GroupStats struct {
	Group
	InstanceCount int
	RunningCount  int
	TransitCount  int
	ErrorCount    int
}

// AggregateState 计算 GroupStats 对应的聚合状态字符串，与前端枚举（spec §7）对齐。
func (s GroupStats) AggregateState() string {
	switch {
	case s.ErrorCount > 0:
		return "error"
	case s.TransitCount > 0:
		return "transitioning"
	case s.InstanceCount == 0, s.RunningCount == 0:
		return "all_stopped"
	case s.RunningCount == s.InstanceCount:
		return "all_running"
	default:
		return "partial"
	}
}

// ValidateName 校验分组名是否符合 1-32 字符的规则。
func ValidateName(name string) error {
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 32 {
		return ErrInvalidName
	}
	if !nameRe.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}
