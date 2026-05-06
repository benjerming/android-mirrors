// Package config 负责加载和校验后端运行配置。
//
// 配置来源优先级：默认值 < YAML 文件 < 环境变量（前缀 APP_）。
// 这样既方便本地写死，也允许部署阶段用环境变量覆盖敏感字段（典型如 session.secret）。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 表示后端运行配置的总入口，把阶段 1 会用到的配置集中收口。
type Config struct {
	App           AppConfig      `mapstructure:"app"`            // App 表示应用自身配置，比如名字、环境和端口。
	Database      DatabaseConfig `mapstructure:"database"`       // Database 表示 SQLite 文件位置等持久化配置。
	Session       SessionConfig  `mapstructure:"session"`        // Session 表示会话签名相关配置。
	Templates     []TemplateSeed `mapstructure:"templates"`      // Templates 表示启动时要预置的模板列表（YAML 内联）。
	TemplatesFile string         `mapstructure:"templates_file"` // TemplatesFile 指向 configs/config.json，用 avdProfiles 覆盖 Templates。
	Android       AndroidConfig  `mapstructure:"android"`        // Android 表示 aapt2 / adb 等外部工具路径。
	Paths         PathsConfig    `mapstructure:"paths"`          // Paths 表示 artifact / file 上传保存目录与允许的设备目录。
	Cleanup       CleanupConfig  `mapstructure:"cleanup"`        // Cleanup 表示周期清理任务的频率与阈值。
}

// AndroidConfig 表示 Android 相关外部工具路径，全部允许为空（fallback 到默认查找逻辑）。
type AndroidConfig struct {
	Aapt2Path string `mapstructure:"aapt2_path"`
	AdbPath   string `mapstructure:"adb_path"`
	ScrcpyPath string `mapstructure:"scrcpy_path"`
}

// PathsConfig 表示后端写入文件的目录约束。
type PathsConfig struct {
	ArtifactRoot     string `mapstructure:"artifact_root"`      // ArtifactRoot 保存 APK 上传后的本地路径前缀。
	AllowedDeviceDir string `mapstructure:"allowed_device_dir"` // AllowedDeviceDir 设备端允许写入的目录前缀。
}

// CleanupConfig 表示周期清理任务的参数。
type CleanupConfig struct {
	SessionInactiveTTLSeconds int `mapstructure:"session_inactive_ttl_seconds"`
	IntervalSeconds           int `mapstructure:"interval_seconds"`
}

// AppConfig 表示应用基础配置，主要控制服务名、环境和监听端口。
type AppConfig struct {
	Name     string `mapstructure:"name"`      // Name 表示应用名，便于日志和运行信息展示。
	Env      string `mapstructure:"env"`       // Env 表示当前环境，用来决定日志输出风格和 Gin 模式。
	HTTPPort int    `mapstructure:"http_port"` // HTTPPort 表示 HTTP 服务监听端口。
}

// DatabaseConfig 表示数据库配置，当前阶段只需要 SQLite 文件路径。
type DatabaseConfig struct {
	Path string `mapstructure:"path"` // Path 表示 SQLite 数据文件保存在哪里。
}

// SessionConfig 表示会话相关配置，目前只放 token 的签名密钥。
//
// Secret 用来对会话 token 做 HMAC 签名：
//   - 不写进配置文件就启动会直接报错，避免不小心用空字符串签名。
//   - 生产环境推荐用环境变量 APP_SESSION_SECRET 覆盖，避免明文落进版本库。
type SessionConfig struct {
	Secret string `mapstructure:"secret"` // Secret 表示用于签发/校验 token 的密钥，必须配置。
}

// TemplateSeed 表示启动时要灌入数据库的一条默认模板配置。
type TemplateSeed struct {
	Name        string `mapstructure:"name"`         // Name 表示模板名称。
	Description string `mapstructure:"description"`  // Description 表示模板说明。
	SystemImage string `mapstructure:"system_image"` // SystemImage 表示 Android system image 标识。
	Device      string `mapstructure:"device"`       // Device 表示 avdmanager 创建 AVD 时使用的设备型号。
	Resolution  string `mapstructure:"resolution"`   // Resolution 表示模拟器屏幕分辨率。
	Density     int    `mapstructure:"density"`      // Density 表示屏幕像素密度（DPI）。
}

// avdProfilesFile 表示 configs/config.json 的最小子集，仅取 avdProfiles + 全局 systemImage。
type avdProfilesFile struct {
	SystemImage string       `json:"systemImage"`
	AVDProfiles []avdProfile `json:"avdProfiles"`
}

// avdProfile 表示 config.json 里单条 AVD 模板，对应前端建机弹窗里的一项。
type avdProfile struct {
	ID          string `json:"id"`
	Device      string `json:"device"`
	DisplayName string `json:"displayName"`
	Resolution  string `json:"resolution"`
	Density     int    `json:"density"`
}

// LoadTemplatesFromFile 用来读取 configs/config.json，把 avdProfiles 转成 TemplateSeed。
//
// path 为空表示没配置外部模板文件，直接返回 nil；这样调用方可以无条件调用，再根据返回值判断要不要替换 Templates。
func LoadTemplatesFromFile(path string) ([]TemplateSeed, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("templates file abs path: %w", err)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read templates file %s: %w", abs, err)
	}

	var parsed avdProfilesFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode templates file %s: %w", abs, err)
	}

	if len(parsed.AVDProfiles) == 0 {
		return nil, nil
	}

	seeds := make([]TemplateSeed, 0, len(parsed.AVDProfiles))
	for _, profile := range parsed.AVDProfiles {
		name := strings.TrimSpace(profile.ID)
		if name == "" {
			continue
		}

		display := strings.TrimSpace(profile.DisplayName)
		descriptionParts := []string{}
		if display != "" && display != name {
			descriptionParts = append(descriptionParts, display)
		}
		if profile.Device != "" {
			descriptionParts = append(descriptionParts, fmt.Sprintf("device=%s", profile.Device))
		}
		if profile.Resolution != "" {
			descriptionParts = append(descriptionParts, fmt.Sprintf("res=%s", profile.Resolution))
		}
		if profile.Density > 0 {
			descriptionParts = append(descriptionParts, fmt.Sprintf("dpi=%d", profile.Density))
		}

		seeds = append(seeds, TemplateSeed{
			Name:        name,
			Description: strings.Join(descriptionParts, " · "),
			SystemImage: parsed.SystemImage,
			Device:      profile.Device,
			Resolution:  profile.Resolution,
			Density:     profile.Density,
		})
	}

	return seeds, nil
}

// Load 用来读取 YAML 默认值和环境变量覆盖，生成应用启动需要的完整配置。
//
// 读取顺序大致是：
//  1. 先把 viper 的默认值（应用名、端口、SQLite 路径等）准备好。
//  2. 读 YAML 配置文件，覆盖默认值。
//  3. 用 APP_XXX 形式的环境变量再覆盖一遍（点号会被替换成下划线）。
//  4. 反序列化进 Config 结构，并校验必填字段。
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值兜底，保证就算某个字段没写进 YAML，也能正常启动。
	v.SetDefault("app.name", "assassin-android-controller")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.http_port", 8080)
	v.SetDefault("database.path", "data/assassin-controller.db")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// session.secret 必须显式配置：空字符串会导致 token 失去防伪造意义，宁可启动失败也别静默放行。
	if strings.TrimSpace(cfg.Session.Secret) == "" {
		return nil, errors.New("config: session.secret is required (set in YAML or via APP_SESSION_SECRET)")
	}

	// templates_file 是相对路径时，按 YAML 文件所在目录解析，所以 configs/config.json 直接写文件名即可。
	if cfg.TemplatesFile != "" && !filepath.IsAbs(cfg.TemplatesFile) {
		cfg.TemplatesFile = filepath.Join(filepath.Dir(configPath), cfg.TemplatesFile)
	}

	if seeds, err := LoadTemplatesFromFile(cfg.TemplatesFile); err != nil {
		return nil, err
	} else if len(seeds) > 0 {
		cfg.Templates = seeds
	}

	return &cfg, nil
}
