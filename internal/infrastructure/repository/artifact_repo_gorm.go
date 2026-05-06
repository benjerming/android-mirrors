package repository

import (
	"context"
	"errors"

	"assassin-android-controller/internal/domain/artifact"
	domainrepo "assassin-android-controller/internal/domain/repository"

	"gorm.io/gorm"
)

// ArtifactRepoGorm 表示基于 GORM 的 artifact 仓储。
type ArtifactRepoGorm struct{ db *gorm.DB }

// NewArtifactRepository 构造 ArtifactRepoGorm。
func NewArtifactRepository(db *gorm.DB) *ArtifactRepoGorm { return &ArtifactRepoGorm{db: db} }

func (r *ArtifactRepoGorm) toDomain(m ArtifactModel) artifact.Artifact {
	return artifact.Artifact{
		ID:          m.ID,
		UserID:      m.UserID,
		Type:        artifact.Type(m.Type),
		OriginName:  m.OriginName,
		Path:        m.Path,
		Size:        m.Size,
		SHA256:      m.SHA256,
		PackageName: m.PackageName,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// List 按 user_id + type 倒序返回 artifact 列表。
func (r *ArtifactRepoGorm) List(ctx context.Context, userID uint, t artifact.Type) ([]artifact.Artifact, error) {
	var rows []ArtifactModel
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if t != "" {
		q = q.Where("type = ?", string(t))
	}
	if err := q.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]artifact.Artifact, len(rows))
	for i, m := range rows {
		out[i] = r.toDomain(m)
	}
	return out, nil
}

// UpsertByOriginName 按 (user_id, type, origin_name) 唯一性做覆盖式入库（spec §3.3.4 同名覆盖）。
func (r *ArtifactRepoGorm) UpsertByOriginName(ctx context.Context, entity *artifact.Artifact) error {
	var existing ArtifactModel
	err := r.db.WithContext(ctx).Where("user_id = ? AND type = ? AND origin_name = ?",
		entity.UserID, string(entity.Type), entity.OriginName).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		m := ArtifactModel{
			UserID:      entity.UserID,
			Type:        string(entity.Type),
			OriginName:  entity.OriginName,
			Path:        entity.Path,
			Size:        entity.Size,
			SHA256:      entity.SHA256,
			PackageName: entity.PackageName,
		}
		if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
			return err
		}
		entity.ID = m.ID
		entity.CreatedAt = m.CreatedAt
		entity.UpdatedAt = m.UpdatedAt
		return nil
	}
	updates := map[string]any{
		"path":         entity.Path,
		"size":         entity.Size,
		"sha256":       entity.SHA256,
		"package_name": entity.PackageName,
	}
	if err := r.db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
		return err
	}
	entity.ID = existing.ID
	entity.CreatedAt = existing.CreatedAt
	return nil
}

// FindByID 校验归属并返回 artifact。
func (r *ArtifactRepoGorm) FindByID(ctx context.Context, userID, id uint) (*artifact.Artifact, error) {
	var m ArtifactModel
	err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainrepo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a := r.toDomain(m)
	return &a, nil
}
