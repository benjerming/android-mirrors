package repository

import (
	"context"
	"errors"

	domainrepo "assassin-android-controller/internal/domain/repository"
	"assassin-android-controller/internal/domain/user"

	"gorm.io/gorm"
)

// UserRepositoryGORM 表示基于 GORM 的用户仓储实现，负责把用户数据写进 SQLite。
type UserRepositoryGORM struct {
	db *gorm.DB // db 表示当前仓储使用的数据库连接。
}

// NewUserRepositoryGORM 用来创建用户仓储实现，通常在启动装配阶段调用。
func NewUserRepositoryGORM(db *gorm.DB) *UserRepositoryGORM {
	return &UserRepositoryGORM{db: db}
}

// FindByID 用来按用户编号查人，给 token 自检和鉴权中间件复用。
func (r *UserRepositoryGORM) FindByID(ctx context.Context, id uint) (*user.User, error) {
	var model UserModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainrepo.ErrNotFound
		}
		return nil, err
	}

	entity := model.toDomain()
	return &entity, nil
}

// FindByUsername 用来按用户名查人，支持“同名直接复用”的登录规则。
func (r *UserRepositoryGORM) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	var model UserModel
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainrepo.ErrNotFound
		}
		return nil, err
	}

	entity := model.toDomain()
	return &entity, nil
}

// Create 用来新增用户记录，通常只会在首次登录时调用。
func (r *UserRepositoryGORM) Create(ctx context.Context, entity *user.User) error {
	model := UserModel{
		Username: entity.Username,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}

	entity.ID = model.ID
	entity.CreatedAt = model.CreatedAt
	entity.UpdatedAt = model.UpdatedAt

	return nil
}
