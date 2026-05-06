package repository

import (
	"context"
	"errors"

	"assassin-android-controller/internal/domain/emulator"
	domainrepo "assassin-android-controller/internal/domain/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TemplateRepositoryGORM 表示基于 GORM 的模板仓储实现，负责读取预置建机模板。
type TemplateRepositoryGORM struct {
	db *gorm.DB // db 表示当前仓储使用的数据库连接。
}

// NewTemplateRepositoryGORM 用来创建模板仓储实现，通常在应用启动时装配。
func NewTemplateRepositoryGORM(db *gorm.DB) *TemplateRepositoryGORM {
	return &TemplateRepositoryGORM{db: db}
}

// List 用来读取全部模板，给前端新建实例弹窗展示选项。
func (r *TemplateRepositoryGORM) List(ctx context.Context) ([]emulator.Template, error) {
	var models []TemplateModel
	if err := r.db.WithContext(ctx).Order("id asc").Find(&models).Error; err != nil {
		return nil, err
	}

	items := make([]emulator.Template, 0, len(models))
	for _, model := range models {
		items = append(items, model.toDomain())
	}

	return items, nil
}

// FindByID 用来校验前端选择的模板是否真实存在。
func (r *TemplateRepositoryGORM) FindByID(ctx context.Context, id uint) (*emulator.Template, error) {
	var model TemplateModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainrepo.ErrNotFound
		}
		return nil, err
	}

	entity := model.toDomain()
	return &entity, nil
}

// Create 用来插入模板记录，主要给启动预置和测试造数使用。
func (r *TemplateRepositoryGORM) Create(ctx context.Context, entity *emulator.Template) error {
	model := TemplateModel{
		Name:        entity.Name,
		Description: entity.Description,
		SystemImage: entity.SystemImage,
		Device:      entity.Device,
		Resolution:  entity.Resolution,
		Density:     entity.Density,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}

	entity.ID = model.ID
	entity.CreatedAt = model.CreatedAt
	entity.UpdatedAt = model.UpdatedAt

	return nil
}

// UpsertByName 按模板 Name 做幂等写入：name 已存在则更新 description / system_image，否则插入。
//
// 启动期 seed 复用这个能力，让 config.json 的改动每次启动都能落库，而不会报唯一索引冲突。
func (r *TemplateRepositoryGORM) UpsertByName(ctx context.Context, entity *emulator.Template) error {
	model := TemplateModel{
		Name:        entity.Name,
		Description: entity.Description,
		SystemImage: entity.SystemImage,
		Device:      entity.Device,
		Resolution:  entity.Resolution,
		Density:     entity.Density,
	}
	tx := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"description", "system_image", "device", "resolution", "density", "updated_at"}),
	}).Create(&model)
	if tx.Error != nil {
		return tx.Error
	}

	// upsert 后 model.ID 在 update 分支可能为 0，需要再查一次拿到稳定的主键。
	if model.ID == 0 {
		if err := r.db.WithContext(ctx).Where("name = ?", entity.Name).First(&model).Error; err != nil {
			return err
		}
	}

	entity.ID = model.ID
	entity.CreatedAt = model.CreatedAt
	entity.UpdatedAt = model.UpdatedAt

	return nil
}
