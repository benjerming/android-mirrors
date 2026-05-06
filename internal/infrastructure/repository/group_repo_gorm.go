package repository

import (
	"context"
	"errors"
	"time"

	"assassin-android-controller/internal/domain/group"
	"assassin-android-controller/internal/domain/repository"

	"gorm.io/gorm"
)

// GroupRepoGorm 表示 GroupRepository 的 GORM 实现。
type GroupRepoGorm struct{ db *gorm.DB }

// NewGroupRepository 用来构造 GroupRepoGorm。
func NewGroupRepository(db *gorm.DB) *GroupRepoGorm { return &GroupRepoGorm{db: db} }

func (r *GroupRepoGorm) toDomain(m GroupModel) group.Group {
	return group.Group{
		ID:        m.ID,
		UserID:    m.UserID,
		Name:      m.Name,
		ProfileID: m.ProfileID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// Create 写入一条分组记录，并把生成的 ID/时间戳回填到入参。
func (r *GroupRepoGorm) Create(ctx context.Context, g *group.Group) error {
	m := GroupModel{UserID: g.UserID, Name: g.Name, ProfileID: g.ProfileID}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	g.ID = m.ID
	g.CreatedAt = m.CreatedAt
	g.UpdatedAt = m.UpdatedAt
	return nil
}

// List 按 created_at 倒序返回当前用户的全部分组。
func (r *GroupRepoGorm) List(ctx context.Context, userID uint) ([]group.Group, error) {
	var rows []GroupModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]group.Group, len(rows))
	for i, m := range rows {
		out[i] = r.toDomain(m)
	}
	return out, nil
}

// ListWithStats 一次性把分组与其下属实例的聚合计数读出来，避免列表页 N+1。
//
// 使用 LEFT JOIN，让"刚建好还没建实例"的分组也能出现在结果里（counts 全 0）。
func (r *GroupRepoGorm) ListWithStats(ctx context.Context, userID uint) ([]group.GroupStats, error) {
	type row struct {
		ID            uint
		UserID        uint
		Name          string
		ProfileID     string
		CreatedAt     time.Time
		UpdatedAt     time.Time
		InstanceCount int
		RunningCount  int
		TransitCount  int
		ErrorCount    int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("group_models AS g").
		Select(`g.id, g.user_id, g.name, g.profile_id, g.created_at, g.updated_at,
            COUNT(i.id) AS instance_count,
            SUM(CASE WHEN i.status = 'running'  THEN 1 ELSE 0 END) AS running_count,
            SUM(CASE WHEN i.status IN ('starting','stopping') THEN 1 ELSE 0 END) AS transit_count,
            SUM(CASE WHEN i.status = 'error'    THEN 1 ELSE 0 END) AS error_count`).
		Joins("LEFT JOIN instance_models i ON i.group_id = g.id").
		Where("g.user_id = ?", userID).
		Group("g.id").
		Order("g.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]group.GroupStats, len(rows))
	for i, m := range rows {
		out[i] = group.GroupStats{
			Group: group.Group{
				ID: m.ID, UserID: m.UserID, Name: m.Name, ProfileID: m.ProfileID,
				CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
			},
			InstanceCount: m.InstanceCount,
			RunningCount:  m.RunningCount,
			TransitCount:  m.TransitCount,
			ErrorCount:    m.ErrorCount,
		}
	}
	return out, nil
}

// FindOwnedByID 校验归属并返回单个分组；非自己分组也返回 ErrNotFound（避免泄漏 ID 存在性）。
func (r *GroupRepoGorm) FindOwnedByID(ctx context.Context, userID, groupID uint) (*group.Group, error) {
	var m GroupModel
	err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, groupID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g := r.toDomain(m)
	return &g, nil
}

// FindByName 按 (userID, name) 查重；缺失返回 ErrNotFound。
func (r *GroupRepoGorm) FindByName(ctx context.Context, userID uint, name string) (*group.Group, error) {
	var m GroupModel
	err := r.db.WithContext(ctx).Where("user_id = ? AND name = ?", userID, name).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g := r.toDomain(m)
	return &g, nil
}

// UpdateName 改名，遇到目标不存在返回 ErrNotFound。
func (r *GroupRepoGorm) UpdateName(ctx context.Context, userID, groupID uint, name string) error {
	res := r.db.WithContext(ctx).Model(&GroupModel{}).Where("user_id = ? AND id = ?", userID, groupID).Update("name", name)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// AttachInstanceForTest 用于测试场景下把已有 instance 行挂到指定 group 下。
//
// 当前阶段 InstanceRepository 还没暴露 GroupID/OwnerUID 写入接口（B-12 起会补上），
// 测试 B-10 聚合查询时需要一个只在测试里使用的入口。命名带 ForTest 前缀以示意。
func (r *GroupRepoGorm) AttachInstanceForTest(ctx context.Context, instanceID, groupID, userID uint) error {
	return r.db.WithContext(ctx).Model(&InstanceModel{}).
		Where("id = ?", instanceID).
		Updates(map[string]any{"group_id": groupID, "owner_uid": userID}).Error
}

// Delete 级联删除：先删该 group 下的全部 instances，再删 group 本身。
func (r *GroupRepoGorm) Delete(ctx context.Context, userID, groupID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ? AND owner_uid = ?", groupID, userID).Delete(&InstanceModel{}).Error; err != nil {
			return err
		}
		res := tx.Where("user_id = ? AND id = ?", userID, groupID).Delete(&GroupModel{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return nil
	})
}
