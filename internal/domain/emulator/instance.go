// Package emulator 定义 Android 模拟器领域里的核心概念，比如模板（Template）和实例（Instance）。
//
// 这一层只放纯粹的领域模型与值对象，不依赖数据库、HTTP 框架等外部技术细节，
// 让业务规则可以独立于基础设施被理解和测试。
package emulator

import "time"

// InstanceMode 表示实例退出登录后的保留策略，让后续清理服务能区分处理方式。
type InstanceMode string

const (
	// InstanceModeReusable 表示实例可重复使用，退出后只需要停机。
	InstanceModeReusable InstanceMode = "reusable"
	// InstanceModeEphemeral 表示实例是临时资源，退出后会延迟删除。
	InstanceModeEphemeral InstanceMode = "ephemeral"
	// InstanceModeDebug 表示实例主要用于调试，退出后会更快删除。
	InstanceModeDebug InstanceMode = "debug"
)

// InstanceStatus 表示实例当前处于什么状态，前端卡片会按它显示不同徽标。
type InstanceStatus string

const (
	// InstanceStatusStopped 表示实例当前没有运行。
	InstanceStatusStopped InstanceStatus = "stopped"
	// InstanceStatusRunning 表示实例已经可以被镜像和控制。
	InstanceStatusRunning InstanceStatus = "running"
	// InstanceStatusStarting 表示实例正在启动（runner.Start 尚未完成 boot）。
	// 该状态只在单个 server 进程生命周期内有意义；server 启停会被无条件清回 stopped。
	InstanceStatusStarting InstanceStatus = "starting"
	// InstanceStatusStopping 表示实例正在停止（runner.Stop 尚未返回）。
	InstanceStatusStopping InstanceStatus = "stopping"
	// InstanceStatusError 表示运行期内某次 runner.Start/Stop 失败。
	// 同样不跨 server 进程持久化，重启后会被刷成 stopped。
	InstanceStatusError InstanceStatus = "error"
)

// Instance 表示一台属于某个用户的 Android 实例，是列表页和镜像页的核心对象。
type Instance struct {
	ID         uint           // ID 表示实例在数据库里的唯一编号。
	Name       string         // Name 表示实例完整名称，会带上归属用户编号与标签。
	Tag        string         // Tag 表示用户输入的实例标签（旧字段，仅 Source="manual" 时填）。
	Mode       InstanceMode   // Mode 表示实例的保留模式，决定退出登录后的清理策略。
	Status     InstanceStatus // Status 表示实例当前运行状态，前端会据此决定是否允许进入镜像。
	UserID     uint           // UserID 表示实例归属用户（旧字段，与 OwnerUID 同值，保留兼容）。
	TemplateID uint           // TemplateID 表示模板编号（仅 Source="manual" 时填）。
	Serial     string         // Serial 表示模拟器启动后对应的 adb 序列号（如 emulator-5554），停机后置空。
	GroupID    uint           // GroupID 表示所属分组（Source="group" 时设置）。
	OwnerUID   uint           // OwnerUID 与 UserID 等价，分组场景下使用此字段表达归属。
	Language   string         // Language 表示 BCP-47 语言代码（分组实例必填）。
	ProfileID  string         // ProfileID 表示模板 ID 字符串，避免再 JOIN groups 表。
	Source     string         // Source 表示创建来源：manual / group / debug。
	CreatedAt  time.Time      // CreatedAt 表示实例记录的创建时间。
	UpdatedAt  time.Time      // UpdatedAt 表示实例记录的最近更新时间。
}
