package group_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"assassin-android-controller/internal/application/group"
	domgroup "assassin-android-controller/internal/domain/group"
	"assassin-android-controller/internal/domain/repository"
)

// fakeRepo 把 GroupRepository 用内存切片实现，避免拉起 GORM。
type fakeRepo struct {
	groups []domgroup.Group
	nextID uint
}

func (f *fakeRepo) Create(_ context.Context, g *domgroup.Group) error {
	f.nextID++
	g.ID = f.nextID
	f.groups = append(f.groups, *g)
	return nil
}

func (f *fakeRepo) List(_ context.Context, userID uint) ([]domgroup.Group, error) {
	out := make([]domgroup.Group, 0, len(f.groups))
	for _, g := range f.groups {
		if g.UserID == userID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListWithStats(_ context.Context, userID uint) ([]domgroup.GroupStats, error) {
	out := make([]domgroup.GroupStats, 0, len(f.groups))
	for _, g := range f.groups {
		if g.UserID == userID {
			out = append(out, domgroup.GroupStats{Group: g})
		}
	}
	return out, nil
}

func (f *fakeRepo) FindOwnedByID(_ context.Context, userID, groupID uint) (*domgroup.Group, error) {
	for i := range f.groups {
		if f.groups[i].UserID == userID && f.groups[i].ID == groupID {
			g := f.groups[i]
			return &g, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeRepo) FindByName(_ context.Context, userID uint, name string) (*domgroup.Group, error) {
	for i := range f.groups {
		if f.groups[i].UserID == userID && f.groups[i].Name == name {
			g := f.groups[i]
			return &g, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeRepo) UpdateName(_ context.Context, userID, groupID uint, name string) error {
	for i := range f.groups {
		if f.groups[i].UserID == userID && f.groups[i].ID == groupID {
			f.groups[i].Name = name
			return nil
		}
	}
	return repository.ErrNotFound
}

func (f *fakeRepo) Delete(_ context.Context, userID, groupID uint) error {
	for i := range f.groups {
		if f.groups[i].UserID == userID && f.groups[i].ID == groupID {
			f.groups = append(f.groups[:i], f.groups[i+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

// fakeCfg 让所有 profile / language 都视为合法，方便聚焦在分组逻辑上。
type fakeCfg struct {
	profiles  map[string]bool
	languages map[string]bool
}

func newFakeCfg() *fakeCfg {
	return &fakeCfg{
		profiles:  map[string]bool{"medium_phone": true, "small_phone": true},
		languages: map[string]bool{"zh-CN": true, "en-US": true, "ja-JP": true},
	}
}

func (f *fakeCfg) HasProfile(id string) bool   { return f.profiles[id] }
func (f *fakeCfg) HasLanguage(code string) bool { return f.languages[code] }

func TestService_Create_RejectsDuplicateName(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)

	if _, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"en-US"},
	})
	if !errors.Is(err, group.ErrNameTaken) {
		t.Errorf("expect ErrNameTaken, got %v", err)
	}
}

func TestService_Create_AllowsSameNameAcrossUsers(t *testing.T) {
	repo := &fakeRepo{}
	svc := group.NewService(repo, newFakeCfg(), nil)

	if _, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	}); err != nil {
		t.Fatalf("user1 create: %v", err)
	}
	if _, err := svc.Create(context.Background(), 2, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	}); err != nil {
		t.Errorf("user2 create: %v", err)
	}
}

func TestService_Create_RejectsInvalidProfile(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	_, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "phantom", Languages: []string{"zh-CN"},
	})
	if !errors.Is(err, group.ErrInvalidProfile) {
		t.Errorf("expect ErrInvalidProfile, got %v", err)
	}
}

func TestService_Create_RejectsEmptyLanguages(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	_, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: nil,
	})
	if !errors.Is(err, group.ErrInvalidLang) {
		t.Errorf("expect ErrInvalidLang, got %v", err)
	}
}

func TestService_Create_RejectsInvalidLanguage(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	_, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN", "xx-YY"},
	})
	if !errors.Is(err, group.ErrInvalidLang) {
		t.Errorf("expect ErrInvalidLang, got %v", err)
	}
}

func TestService_Create_RejectsInvalidName(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	_, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组 A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	})
	if !errors.Is(err, domgroup.ErrInvalidName) {
		t.Errorf("expect ErrInvalidName, got %v", err)
	}
}

func TestService_Rename_DetectsConflict(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)

	res1, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	})
	if err != nil {
		t.Fatalf("create g1: %v", err)
	}
	if _, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组B", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	}); err != nil {
		t.Fatalf("create g2: %v", err)
	}

	if err := svc.Rename(context.Background(), 1, res1.Group.ID, "组B"); !errors.Is(err, group.ErrNameTaken) {
		t.Errorf("expect ErrNameTaken, got %v", err)
	}
	if err := svc.Rename(context.Background(), 1, res1.Group.ID, "组A2"); err != nil {
		t.Errorf("rename to fresh name: %v", err)
	}
}

func TestService_Rename_NotFoundForOtherUser(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	res1, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Rename(context.Background(), 99, res1.Group.ID, "组C"); !errors.Is(err, group.ErrNotFound) {
		t.Errorf("expect ErrNotFound, got %v", err)
	}
}

func TestService_Delete_NotFoundForOtherUser(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	res1, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(context.Background(), 99, res1.Group.ID); !errors.Is(err, group.ErrNotFound) {
		t.Errorf("expect ErrNotFound, got %v", err)
	}
	if err := svc.Delete(context.Background(), 1, res1.Group.ID); err != nil {
		t.Errorf("delete own: %v", err)
	}
}

func TestService_List_OnlyOwnUser(t *testing.T) {
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), nil)
	for _, name := range []string{"组A", "组B"} {
		if _, err := svc.Create(context.Background(), 1, group.CreateInput{
			Name: name, ProfileID: "medium_phone", Languages: []string{"zh-CN"},
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if _, err := svc.Create(context.Background(), 2, group.CreateInput{
		Name: "组X", ProfileID: "medium_phone", Languages: []string{"zh-CN"},
	}); err != nil {
		t.Fatalf("seed user2: %v", err)
	}

	gs, err := svc.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(gs) != 2 {
		t.Errorf("expect 2 groups for user1, got %d", len(gs))
	}
}

// fakeMaker 实现 InstanceCreator，按预设结果返回成功/失败。
type fakeMaker struct {
	mu        sync.Mutex
	idCounter uint
	failOn    map[string]bool // map[language]bool
	calls     int
}

func (f *fakeMaker) CreateForGroup(_ context.Context, _ uint, _ uint, _ string, language string) (uint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failOn[language] {
		return 0, errors.New("avd failed")
	}
	f.idCounter++
	return f.idCounter, nil
}

func TestService_Create_Concurrent_AllSuccess(t *testing.T) {
	maker := &fakeMaker{}
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), maker)
	res, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN", "en-US", "ja-JP"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(res.InstanceIDs) != 3 {
		t.Errorf("expect 3 instance ids, got %d", len(res.InstanceIDs))
	}
	if len(res.Failed) != 0 {
		t.Errorf("expect 0 failed, got %d", len(res.Failed))
	}
}

func TestService_Create_Concurrent_PartialFailure(t *testing.T) {
	maker := &fakeMaker{failOn: map[string]bool{"en-US": true}}
	svc := group.NewService(&fakeRepo{}, newFakeCfg(), maker)
	res, err := svc.Create(context.Background(), 1, group.CreateInput{
		Name: "组A", ProfileID: "medium_phone", Languages: []string{"zh-CN", "en-US", "ja-JP"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(res.InstanceIDs) != 2 {
		t.Errorf("expect 2 successful, got %d", len(res.InstanceIDs))
	}
	if len(res.Failed) != 1 || res.Failed[0].Language != "en-US" {
		t.Errorf("expect failed[en-US], got %+v", res.Failed)
	}
	// 分组本身仍然落库。
	if res.Group.ID == 0 {
		t.Error("group should still be persisted on partial failure")
	}
}
