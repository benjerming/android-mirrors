// Package repository 定义领域层向外暴露的"持久化入口"接口。
//
// 这里只声明仓储接口和领域级错误（比如 ErrNotFound），具体的 SQL 实现放在
// internal/infrastructure/repository 里。这样应用服务只依赖接口、不依赖 GORM，
// 既方便单元测试用 fake 替换，也方便以后切到别的存储后端。
package repository

import (
	"context"
	"errors"

	"assassin-android-controller/internal/domain/artifact"
	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/group"
	"assassin-android-controller/internal/domain/user"
)

// ErrNotFound 表示仓储里没有找到目标数据，业务层会把它翻译成 404 或权限失败。
var ErrNotFound = errors.New("repository: not found")

// UserRepository 表示用户数据的读取与保存入口，应用服务通过它管理登录用户。
type UserRepository interface {
	FindByID(ctx context.Context, id uint) (*user.User, error)
	FindByUsername(ctx context.Context, username string) (*user.User, error)
	Create(ctx context.Context, entity *user.User) error
}

// TemplateRepository 表示模板数据的读取与保存入口，建机和模板列表都会用到它。
type TemplateRepository interface {
	List(ctx context.Context) ([]emulator.Template, error)
	FindByID(ctx context.Context, id uint) (*emulator.Template, error)
	Create(ctx context.Context, entity *emulator.Template) error
}

// InstanceRepository 表示实例数据的读取与保存入口，列表页和实例操作都依赖它。
//
// FindByID 与 FindOwnedByID 的分工：
//   - FindByID 用来查"任意用户名下的某条实例记录"，主要服务于"区分 404 和 403"的场景。
//   - FindOwnedByID 用来查"某个用户名下的某条实例记录"，是日常归属校验的主入口。
type InstanceRepository interface {
	ListByUserID(ctx context.Context, userID uint) ([]emulator.Instance, error)
	ListByGroup(ctx context.Context, userID uint, groupID uint) ([]emulator.Instance, error)
	FindByID(ctx context.Context, instanceID uint) (*emulator.Instance, error)
	FindOwnedByID(ctx context.Context, userID uint, instanceID uint) (*emulator.Instance, error)
	Create(ctx context.Context, entity *emulator.Instance) error
	Update(ctx context.Context, entity *emulator.Instance) error
	Delete(ctx context.Context, userID uint, instanceID uint) error
}

// ArtifactRepository 表示 APK / 资源元数据的入口。
type ArtifactRepository interface {
	List(ctx context.Context, userID uint, t artifact.Type) ([]artifact.Artifact, error)
	UpsertByOriginName(ctx context.Context, entity *artifact.Artifact) error
	FindByID(ctx context.Context, userID, id uint) (*artifact.Artifact, error)
}

// GroupRepository 表示分组数据的读取与保存入口。
type GroupRepository interface {
	List(ctx context.Context, userID uint) ([]group.Group, error)
	ListWithStats(ctx context.Context, userID uint) ([]group.GroupStats, error)
	FindOwnedByID(ctx context.Context, userID uint, groupID uint) (*group.Group, error)
	FindByName(ctx context.Context, userID uint, name string) (*group.Group, error)
	Create(ctx context.Context, entity *group.Group) error
	UpdateName(ctx context.Context, userID uint, groupID uint, name string) error
	Delete(ctx context.Context, userID uint, groupID uint) error
}
