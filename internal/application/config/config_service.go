// Package config 提供配置服务，把 configs/config.json 的 profiles + languages 暴露给应用层。
//
// 启动时一次性把 config.json 缓存到内存。运维改 config 必须重启服务。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"assassin-android-controller/internal/domain/language"
	"assassin-android-controller/internal/domain/profile"
)

// Options 表示前端 GET /configs/options 一次性返回的配置选项。
type Options struct {
	Profiles  []profile.Profile
	Languages []language.Language
}

// Service 表示配置服务，启动时一次性把 config.json 缓存到内存。
type Service struct {
	mu        sync.RWMutex
	profiles  map[string]profile.Profile
	languages map[string]language.Language
	ordered   Options
}

// New 用来从指定路径读取 config.json 并构造 Service。
func New(path string) (*Service, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config.json: %w", err)
	}

	var parsed struct {
		AVDProfiles []profile.Profile   `json:"avdProfiles"`
		Languages   []language.Language `json:"languages"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode config.json: %w", err)
	}

	s := &Service{
		profiles:  make(map[string]profile.Profile, len(parsed.AVDProfiles)),
		languages: make(map[string]language.Language, len(parsed.Languages)),
		ordered:   Options{Profiles: parsed.AVDProfiles, Languages: parsed.Languages},
	}
	for _, p := range parsed.AVDProfiles {
		s.profiles[p.ID] = p
	}
	for _, l := range parsed.Languages {
		s.languages[l.Code] = l
	}
	return s, nil
}

// Options 返回保留原顺序的全部 profiles 与 languages。
func (s *Service) Options() Options {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ordered
}

// HasProfile 判断 profileID 是否合法。
func (s *Service) HasProfile(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.profiles[id]
	return ok
}

// HasLanguage 判断 languageCode 是否合法。
func (s *Service) HasLanguage(code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.languages[code]
	return ok
}

// Profile 用来按 ID 取出 profile（含 Device / Resolution 等用于建 AVD）。
func (s *Service) Profile(id string) (profile.Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[id]
	return p, ok
}
