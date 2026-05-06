// Package repository 提供领域仓储接口的 GORM/SQLite 实现。
//
// 这里同时定义数据库的存储模型（UserModel / TemplateModel / InstanceModel），
// 并在仓储方法里完成"领域对象 ↔ 存储模型"的转换，避免让数据库标签污染领域层。
package repository

import (
	"time"

	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/user"

	"gorm.io/gorm"
)

// UserModel 表示用户在 SQLite 里的存储结构，用来保存会话归属所需的最小信息。
type UserModel struct {
	ID        uint      `gorm:"primaryKey"`          // ID 表示数据库主键，会映射回领域里的用户编号。
	Username  string    `gorm:"uniqueIndex;size:64"` // Username 表示登录用户名，全局唯一。
	CreatedAt time.Time // CreatedAt 表示记录创建时间。
	UpdatedAt time.Time // UpdatedAt 表示记录更新时间。
}

// TemplateModel 表示模拟器模板在数据库里的存储结构，给新建实例弹窗提供数据源。
type TemplateModel struct {
	ID          uint      `gorm:"primaryKey"`           // ID 表示模板主键。
	Name        string    `gorm:"size:128;uniqueIndex"` // Name 表示模板名称，全局唯一以便按名字 upsert。
	Description string    `gorm:"size:255"`             // Description 表示模板说明。
	SystemImage string    `gorm:"size:255"`             // SystemImage 表示 Android system image 标识。
	Device      string    `gorm:"size:64"`              // Device 表示 avdmanager 使用的设备型号。
	Resolution  string    `gorm:"size:32"`              // Resolution 表示模拟器屏幕分辨率。
	Density     int       // Density 表示屏幕像素密度。
	CreatedAt   time.Time // CreatedAt 表示记录创建时间。
	UpdatedAt   time.Time // UpdatedAt 表示记录更新时间。
}

// InstanceModel 表示实例在数据库里的存储结构，是列表页和实例操作的持久化基础。
type InstanceModel struct {
	ID         uint      `gorm:"primaryKey"`     // ID 表示实例主键。
	Name       string    `gorm:"size:255;index"` // Name 表示完整实例名。
	Tag        string    `gorm:"size:128"`       // Tag 表示用户输入的实例标签（旧字段，保留兼容）。
	Mode       string    `gorm:"size:32"`        // Mode 表示实例保留模式。
	Status     string    `gorm:"size:32"`        // Status 表示实例运行状态。
	UserID     uint      `gorm:"index"`          // UserID 表示实例归属用户（旧字段，保留兼容）。
	TemplateID uint      `gorm:"index"`          // TemplateID 表示实例所用模板（旧字段，保留兼容）。
	Serial     string    `gorm:"size:64"`        // Serial 表示运行中的 adb 序列号。
	GroupID    uint      `gorm:"index"`          // GroupID 表示所属分组。
	Language   string    `gorm:"size:16"`        // Language 表示 BCP-47 语言代码。
	ProfileID  string    `gorm:"size:128"`       // ProfileID 表示模板 ID，冗余以避免 JOIN groups。
	OwnerUID   uint      `gorm:"index"`          // OwnerUID 与 group.user_id 冗余。
	Source     string    `gorm:"size:32"`        // Source 表示创建来源（manual / group / debug）。
	SpecJSON   string    `gorm:"type:text"`      // SpecJSON 表示创建时硬件参数快照（JSON）。
	CreatedAt  time.Time // CreatedAt 表示记录创建时间。
	UpdatedAt  time.Time // UpdatedAt 表示记录更新时间。
}

// GroupModel 表示分组在数据库里的存储结构。
type GroupModel struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;uniqueIndex:uniq_user_name"` // 与 Name 组合实现"同用户下唯一"。
	Name      string    `gorm:"size:32;uniqueIndex:uniq_user_name"`
	ProfileID string    `gorm:"size:128"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ArtifactModel 表示 APK / 资源文件上传后的元数据。
type ArtifactModel struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"index"`
	Type        string `gorm:"size:32"`               // Type 表示资源类型，目前固定 "apk"。
	OriginName  string `gorm:"size:255"`              // OriginName 上传时的原始文件名。
	Path        string `gorm:"size:512"`              // Path 服务器侧落地路径（绝对 / 相对于 artifact_root）。
	Size        int64  // Size 字节数。
	SHA256      string `gorm:"size:64;index"`
	PackageName string `gorm:"size:255"` // PackageName aapt2 解析出的包名（仅 type=apk）。
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AutoMigrate 用来在应用启动或测试准备阶段自动建表，避免手工维护阶段 1 的表结构。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&UserModel{}, &TemplateModel{}, &InstanceModel{}, &GroupModel{}, &ArtifactModel{})
}

// toDomain 用来把数据库里的用户记录转换成业务层使用的领域对象。
func (m UserModel) toDomain() user.User {
	return user.User{
		ID:        m.ID,
		Username:  m.Username,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// toDomain 用来把数据库里的模板记录转换成业务层使用的领域对象。
func (m TemplateModel) toDomain() emulator.Template {
	return emulator.Template{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		SystemImage: m.SystemImage,
		Device:      m.Device,
		Resolution:  m.Resolution,
		Density:     m.Density,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// toDomain 用来把数据库里的实例记录转换成业务层使用的领域对象。
func (m InstanceModel) toDomain() emulator.Instance {
	return emulator.Instance{
		ID:         m.ID,
		Name:       m.Name,
		Tag:        m.Tag,
		Mode:       emulator.InstanceMode(m.Mode),
		Status:     emulator.InstanceStatus(m.Status),
		UserID:     m.UserID,
		TemplateID: m.TemplateID,
		Serial:     m.Serial,
		GroupID:    m.GroupID,
		OwnerUID:   m.OwnerUID,
		Language:   m.Language,
		ProfileID:  m.ProfileID,
		Source:     m.Source,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}
