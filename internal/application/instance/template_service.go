package instance

import (
	"context"

	"assassin-android-controller/internal/domain/emulator"
	"assassin-android-controller/internal/domain/repository"
)

// TemplateService 表示模板应用服务，负责给前端返回新建实例所需的模板列表。
type TemplateService struct {
	templateRepo repository.TemplateRepository // templateRepo 用来读取预置模板，避免 handler 直接访问数据库。
}

// NewTemplateService 用来创建模板服务，通常在启动时完成依赖装配。
func NewTemplateService(templateRepo repository.TemplateRepository) *TemplateService {
	return &TemplateService{templateRepo: templateRepo}
}

// List 用来获取当前可用模板列表，给实例创建弹窗的下拉框使用。
func (s *TemplateService) List(ctx context.Context) ([]emulator.Template, error) {
	return s.templateRepo.List(ctx)
}
