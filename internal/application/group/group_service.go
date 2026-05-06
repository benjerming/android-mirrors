// Package group 提供分组的应用用例：Create / List / Rename / Delete。
//
// 本阶段（M3）只覆盖"事务1建 group + 入参校验"，并发建 AVD 留给 B-12 接入；
// 因此 InstanceCreator 接口在这里先声明，调用方传 nil 也能跑通。
package group

import (
	"context"
	"errors"
	"sync"

	domgroup "assassin-android-controller/internal/domain/group"
	"assassin-android-controller/internal/domain/repository"
)

// defaultCreateConcurrency 表示并发建 AVD 的默认上限，参考 spec §4.2 的 group.create_concurrent_limit。
const defaultCreateConcurrency = 20

// 业务级错误：handler 层把它们映射到具体 HTTP 状态码。
var (
	ErrNameTaken      = errors.New("group: name already taken")
	ErrInvalidProfile = errors.New("group: invalid profile_id")
	ErrInvalidLang    = errors.New("group: invalid language")
	ErrNotFound       = errors.New("group: not found")
)

// ConfigOracle 把"profile / language 是否合法"抽象出来，便于注入 fake。
//
// 真实实现由 application/config.Service 提供（已实现 HasProfile / HasLanguage）。
type ConfigOracle interface {
	HasProfile(id string) bool
	HasLanguage(code string) bool
}

// InstanceCreator 表示"为分组并发建 N 台实例"的能力，B-12 接入真实实现，B-7 阶段允许为 nil。
type InstanceCreator interface {
	CreateForGroup(ctx context.Context, userID uint, groupID uint, profileID string, language string) (instanceID uint, err error)
}

// CreateInput 表示创建分组的入参。
type CreateInput struct {
	Name      string
	ProfileID string
	Languages []string
}

// Service 表示分组应用服务。
type Service struct {
	repo        repository.GroupRepository
	cfg         ConfigOracle
	maker       InstanceCreator
	cleaner     GroupCleaner
	concurrency int
}

// GroupCleaner 表示"删除分组前先并发停模拟器 + 删 AVD"的能力。B-14 起注入。
type GroupCleaner interface {
	StopAndDeleteByGroup(ctx context.Context, userID, groupID uint) error
}

// NewService 用来构造 Service。maker / cleaner 在早期阶段允许传 nil。
func NewService(repo repository.GroupRepository, cfg ConfigOracle, maker InstanceCreator) *Service {
	return &Service{repo: repo, cfg: cfg, maker: maker, concurrency: defaultCreateConcurrency}
}

// WithCleaner 注入分组删除前的清理器（B-14 起）。
func (s *Service) WithCleaner(c GroupCleaner) *Service {
	s.cleaner = c
	return s
}

// WithConcurrency 自定义并发建 AVD 的上限，主要给单测控制时序。
func (s *Service) WithConcurrency(n int) *Service {
	if n > 0 {
		s.concurrency = n
	}
	return s
}

// FailedLanguage 表示某个语言对应的实例创建失败。
type FailedLanguage struct {
	Language string
	Error    string
}

// CreateResult 表示创建分组的最终结果，包含分组本身、成功的实例 id 列表与失败语言列表。
type CreateResult struct {
	Group       domgroup.Group
	InstanceIDs []uint
	Failed      []FailedLanguage
}

// Create 实现 spec §4.2：事务1建 group → 并发阶段 N 次建 AVD/instance → 汇总。
//
// 当 maker 为 nil（早期里程碑），并发阶段被跳过、Failed/InstanceIDs 都为空。
func (s *Service) Create(ctx context.Context, userID uint, in CreateInput) (CreateResult, error) {
	var res CreateResult
	if e := domgroup.ValidateName(in.Name); e != nil {
		return res, e
	}
	if !s.cfg.HasProfile(in.ProfileID) {
		return res, ErrInvalidProfile
	}
	if len(in.Languages) == 0 {
		return res, ErrInvalidLang
	}
	for _, l := range in.Languages {
		if !s.cfg.HasLanguage(l) {
			return res, ErrInvalidLang
		}
	}

	if existing, _ := s.repo.FindByName(ctx, userID, in.Name); existing != nil {
		return res, ErrNameTaken
	}

	g := domgroup.Group{UserID: userID, Name: in.Name, ProfileID: in.ProfileID}
	if err := s.repo.Create(ctx, &g); err != nil {
		return res, err
	}
	res.Group = g

	if s.maker == nil {
		return res, nil
	}

	type item struct {
		lang string
		id   uint
		err  error
	}
	out := make(chan item, len(in.Languages))
	limit := s.concurrency
	if limit <= 0 {
		limit = defaultCreateConcurrency
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, l := range in.Languages {
		wg.Add(1)
		sem <- struct{}{}
		go func(lang string) {
			defer wg.Done()
			defer func() { <-sem }()
			id, err := s.maker.CreateForGroup(ctx, userID, g.ID, g.ProfileID, lang)
			out <- item{lang: lang, id: id, err: err}
		}(l)
	}
	wg.Wait()
	close(out)

	for it := range out {
		if it.err != nil {
			res.Failed = append(res.Failed, FailedLanguage{Language: it.lang, Error: it.err.Error()})
		} else {
			res.InstanceIDs = append(res.InstanceIDs, it.id)
		}
	}
	return res, nil
}

// List 返回当前用户的分组（不含聚合状态）。
func (s *Service) List(ctx context.Context, userID uint) ([]domgroup.Group, error) {
	return s.repo.List(ctx, userID)
}

// ListWithStats 返回当前用户的分组连同实例计数，给列表页一次性消费。
func (s *Service) ListWithStats(ctx context.Context, userID uint) ([]domgroup.GroupStats, error) {
	return s.repo.ListWithStats(ctx, userID)
}

// Rename 校验归属并改名，遇重名返回 ErrNameTaken，遇不存在返回 ErrNotFound。
func (s *Service) Rename(ctx context.Context, userID, groupID uint, name string) error {
	if err := domgroup.ValidateName(name); err != nil {
		return err
	}
	if existing, _ := s.repo.FindByName(ctx, userID, name); existing != nil && existing.ID != groupID {
		return ErrNameTaken
	}
	if err := s.repo.UpdateName(ctx, userID, groupID, name); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Delete 级联删除：先并发停模拟器 + 删 AVD（cleaner），再删数据库行。
//
// cleaner 为 nil（早期阶段）时跳过运行时清理，仅删库；保持向后兼容。
func (s *Service) Delete(ctx context.Context, userID, groupID uint) error {
	if s.cleaner != nil {
		if err := s.cleaner.StopAndDeleteByGroup(ctx, userID, groupID); err != nil {
			return err
		}
	}
	if err := s.repo.Delete(ctx, userID, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// StartByGroup 整组启动派发：仅校验归属，把派发动作转给 starter（实际是 InstanceService）。
// 注意：返回时启动还未完成，handler 应该返回 202 让前端轮询 /groups 看 aggregateState。
func (s *Service) StartByGroup(ctx context.Context, userID, groupID uint, starter GroupStarter) (transitioning []uint, skipped []uint, err error) {
	if _, err := s.repo.FindOwnedByID(ctx, userID, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if starter == nil {
		return nil, nil, nil
	}
	return starter.DispatchStartByGroup(ctx, userID, groupID)
}

// StopByGroup 镜像 StartByGroup。
func (s *Service) StopByGroup(ctx context.Context, userID, groupID uint, stopper GroupStopper) (transitioning []uint, skipped []uint, err error) {
	if _, err := s.repo.FindOwnedByID(ctx, userID, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if stopper == nil {
		return nil, nil, nil
	}
	return stopper.DispatchStopByGroup(ctx, userID, groupID)
}

// GroupStarter / GroupStopper 把"整组启停"的派发 + 后台执行能力抽象出来。
// Dispatch* 在事务里把目标行翻成 starting/stopping，立即返回；
// Run*Fanout 由 handler 在 goroutine 里跑，真正调 runner.Start/Stop。
type GroupStarter interface {
	DispatchStartByGroup(ctx context.Context, userID, groupID uint) (transitioning []uint, skipped []uint, err error)
	RunStartFanout(ctx context.Context, ids []uint)
}

type GroupStopper interface {
	DispatchStopByGroup(ctx context.Context, userID, groupID uint) (transitioning []uint, skipped []uint, err error)
	RunStopFanout(ctx context.Context, ids []uint)
}
