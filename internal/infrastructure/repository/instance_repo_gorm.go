package repository

import (
	"context"
	"errors"

	"assassin-android-controller/internal/domain/emulator"
	domainrepo "assassin-android-controller/internal/domain/repository"

	"gorm.io/gorm"
)

// InstanceRepositoryGORM 表示基于 GORM 的实例仓储实现，负责保存实例列表页需要的数据。
type InstanceRepositoryGORM struct {
	db *gorm.DB // db 表示当前仓储使用的数据库连接。
}

// NewInstanceRepositoryGORM 用来创建实例仓储实现，通常在应用启动装配依赖时调用。
func NewInstanceRepositoryGORM(db *gorm.DB) *InstanceRepositoryGORM {
	return &InstanceRepositoryGORM{db: db}
}

// ListByUserID 用来列出某个用户自己的实例，避免跨用户串数据。
func (r *InstanceRepositoryGORM) ListByUserID(ctx context.Context, userID uint) ([]emulator.Instance, error) {
	var models []InstanceModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id asc").Find(&models).Error; err != nil {
		return nil, err
	}

	items := make([]emulator.Instance, 0, len(models))
	for _, model := range models {
		items = append(items, model.toDomain())
	}

	return items, nil
}

// FindByID 用来按主键读取实例记录，不限归属。
//
// 主要服务于"区分实例不存在 vs. 实例存在但不属于当前用户"这类需要返回 403 还是 404 的场景。
// 日常的归属校验请走 FindOwnedByID，避免越权访问被悄悄放过。
func (r *InstanceRepositoryGORM) FindByID(ctx context.Context, instanceID uint) (*emulator.Instance, error) {
	var model InstanceModel
	if err := r.db.WithContext(ctx).First(&model, instanceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainrepo.ErrNotFound
		}
		return nil, err
	}

	entity := model.toDomain()
	return &entity, nil
}

// FindOwnedByID 用来读取“属于某个用户的指定实例”，给权限校验和启停删流程复用。
func (r *InstanceRepositoryGORM) FindOwnedByID(ctx context.Context, userID uint, instanceID uint) (*emulator.Instance, error) {
	var model InstanceModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", instanceID, userID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainrepo.ErrNotFound
		}
		return nil, err
	}

	entity := model.toDomain()
	return &entity, nil
}

// Create 用来保存新建实例记录，让列表页立刻能读到它。
func (r *InstanceRepositoryGORM) Create(ctx context.Context, entity *emulator.Instance) error {
	model := InstanceModel{
		Name:       entity.Name,
		Tag:        entity.Tag,
		Mode:       string(entity.Mode),
		Status:     string(entity.Status),
		UserID:     entity.UserID,
		TemplateID: entity.TemplateID,
		Serial:     entity.Serial,
		GroupID:    entity.GroupID,
		OwnerUID:   entity.OwnerUID,
		Language:   entity.Language,
		ProfileID:  entity.ProfileID,
		Source:     entity.Source,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}

	entity.ID = model.ID
	entity.CreatedAt = model.CreatedAt
	entity.UpdatedAt = model.UpdatedAt

	return nil
}

// Update 用来更新实例状态等字段，支持启动和停止操作。
func (r *InstanceRepositoryGORM) Update(ctx context.Context, entity *emulator.Instance) error {
	updates := map[string]any{
		"name":        entity.Name,
		"tag":         entity.Tag,
		"mode":        string(entity.Mode),
		"status":      string(entity.Status),
		"user_id":     entity.UserID,
		"template_id": entity.TemplateID,
		"serial":      entity.Serial,
		"group_id":    entity.GroupID,
		"owner_uid":   entity.OwnerUID,
		"language":    entity.Language,
		"profile_id":  entity.ProfileID,
		"source":      entity.Source,
	}
	return r.db.WithContext(ctx).Model(&InstanceModel{}).Where("id = ?", entity.ID).Updates(updates).Error
}

// ListByGroup 用来列出某分组下的全部实例（按 id 升序），给整组启停 / 删除复用。
func (r *InstanceRepositoryGORM) ListByGroup(ctx context.Context, userID, groupID uint) ([]emulator.Instance, error) {
	var rows []InstanceModel
	if err := r.db.WithContext(ctx).
		Where("group_id = ? AND owner_uid = ?", groupID, userID).
		Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]emulator.Instance, len(rows))
	for i, m := range rows {
		out[i] = m.toDomain()
	}
	return out, nil
}

// Delete 用来删除某个用户自己的实例记录，避免误删别人的实例。
func (r *InstanceRepositoryGORM) Delete(ctx context.Context, userID uint, instanceID uint) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", instanceID, userID).Delete(&InstanceModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainrepo.ErrNotFound
	}
	return nil
}
