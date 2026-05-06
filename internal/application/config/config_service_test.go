package config

import (
	"path/filepath"
	"testing"
)

// TestLoad_ReadsProfilesAndLanguages 验证从 configs/config.json 读取 profiles + languages。
func TestLoad_ReadsProfilesAndLanguages(t *testing.T) {
	abs, err := filepath.Abs("../../../configs/config.json")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	svc, err := New(abs)
	if err != nil {
		t.Fatal(err)
	}

	opts := svc.Options()
	if len(opts.Profiles) == 0 {
		t.Fatal("expect profiles > 0")
	}
	if len(opts.Languages) == 0 {
		t.Fatal("expect languages > 0")
	}

	if !svc.HasProfile("Mirrors_Xperia_XZ") {
		t.Error("expect profile Mirrors_Xperia_XZ")
	}
	if !svc.HasLanguage("zh-CN") {
		t.Error("expect language zh-CN")
	}
	if svc.HasLanguage("xx-XX") {
		t.Error("xx-XX should not exist")
	}
}
