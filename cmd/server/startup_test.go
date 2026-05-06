package main

import "testing"

// TestResolveConfigPathUsesDefault 用来确认不传任何参数、也没有 APP_CONFIG 环境变量时，
// 启动会回落到编译期常量 defaultConfigPath。
func TestResolveConfigPathUsesDefault(t *testing.T) {
	// t.Setenv 在测试结束时会自动恢复，避免污染同一进程里的其他测试。
	t.Setenv("APP_CONFIG", "")

	configPath, err := resolveConfigPath(nil)
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}

	if configPath != defaultConfigPath {
		t.Fatalf("expected default config path %q, got %q", defaultConfigPath, configPath)
	}
}

// TestResolveConfigPathHonorsEnvVar 用来确认设置了 APP_CONFIG 之后，
// 即便没有 -config 参数也会用环境变量里的路径。
func TestResolveConfigPathHonorsEnvVar(t *testing.T) {
	t.Setenv("APP_CONFIG", "/etc/assassin/from-env.yaml")

	configPath, err := resolveConfigPath(nil)
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}

	if configPath != "/etc/assassin/from-env.yaml" {
		t.Fatalf("expected env var path, got %q", configPath)
	}
}

// TestResolveConfigPathPrefersFlag 用来确认显式传入 -config 时，
// 会盖过默认值和环境变量，让命令行始终拥有最高优先级。
func TestResolveConfigPathPrefersFlag(t *testing.T) {
	t.Setenv("APP_CONFIG", "/etc/assassin/from-env.yaml")

	configPath, err := resolveConfigPath([]string{"-config", "/tmp/custom.yaml"})
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}

	if configPath != "/tmp/custom.yaml" {
		t.Fatalf("expected custom config path, got %q", configPath)
	}
}
