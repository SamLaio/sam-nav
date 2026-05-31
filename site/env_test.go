package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestEnvUsesEmbeddedDefaultFile(t *testing.T) {
	t.Setenv("SAM_NAV_SITE_TITLE", "")
	resetEnvCache()

	if got := env("SAM_NAV_SITE_TITLE", "fallback"); got != "Sam Nav" {
		t.Fatalf("應讀取內建 deffault.env 預設值，取得：%s", got)
	}
	if got := env("SAM_NAV_UNKNOWN_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("未知環境變數應使用 fallback，取得：%s", got)
	}
}

func TestEnvPrefersRuntimeValue(t *testing.T) {
	t.Setenv("SAM_NAV_SITE_TITLE", "Custom Nav")
	resetEnvCache()

	if got := env("SAM_NAV_SITE_TITLE", "fallback"); got != "Custom Nav" {
		t.Fatalf("執行環境變數應覆蓋內建預設值，取得：%s", got)
	}
}

func TestEnvUsesRootSettingFileBeforeEmbeddedDefault(t *testing.T) {
	t.Setenv("SAM_NAV_SITE_TITLE", "")
	rootDir := t.TempDir()
	siteDir := filepath.Join(rootDir, "site")
	if err := os.Mkdir(siteDir, 0o755); err != nil {
		t.Fatalf("建立 site 測試目錄失敗：%v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "setting.env"), []byte("SAM_NAV_SITE_TITLE=Local Nav\nSAM_NAV_PORT=7777\n"), 0o644); err != nil {
		t.Fatalf("寫入 setting.env 失敗：%v", err)
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取得工作目錄失敗：%v", err)
	}
	if err := os.Chdir(siteDir); err != nil {
		t.Fatalf("切換工作目錄失敗：%v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("還原工作目錄失敗：%v", err)
		}
		resetEnvCache()
	})
	resetEnvCache()

	if got := env("SAM_NAV_SITE_TITLE", "fallback"); got != "Local Nav" {
		t.Fatalf("setting.env 應覆蓋內建預設值，取得：%s", got)
	}
	if got := env("SAM_NAV_PORT", "fallback"); got != "7777" {
		t.Fatalf("setting.env 應提供本機 port，取得：%s", got)
	}
}

func TestEnvPrefersRuntimeValueBeforeSettingFile(t *testing.T) {
	t.Setenv("SAM_NAV_SITE_TITLE", "Runtime Nav")
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "setting.env"), []byte("SAM_NAV_SITE_TITLE=Local Nav\n"), 0o644); err != nil {
		t.Fatalf("寫入 setting.env 失敗：%v", err)
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取得工作目錄失敗：%v", err)
	}
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("切換工作目錄失敗：%v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("還原工作目錄失敗：%v", err)
		}
		resetEnvCache()
	})
	resetEnvCache()

	if got := env("SAM_NAV_SITE_TITLE", "fallback"); got != "Runtime Nav" {
		t.Fatalf("執行環境變數應優先於 setting.env，取得：%s", got)
	}
}

func TestParseEnvFile(t *testing.T) {
	values := parseEnvFile(`
# 註解應被略過
SAM_NAV_PORT=6412
SAM_NAV_SITE_TITLE=Sam Nav
SAM_NAV_EMPTY=
`)

	if values["SAM_NAV_PORT"] != "6412" {
		t.Fatalf("解析 SAM_NAV_PORT 失敗：%s", values["SAM_NAV_PORT"])
	}
	if values["SAM_NAV_SITE_TITLE"] != "Sam Nav" {
		t.Fatalf("解析含空白的值失敗：%s", values["SAM_NAV_SITE_TITLE"])
	}
	if value, ok := values["SAM_NAV_EMPTY"]; !ok || value != "" {
		t.Fatalf("空值應被保留，取得：%q，存在：%t", value, ok)
	}
}

func resetEnvCache() {
	defaultEnvOnce = sync.Once{}
	defaultEnv = nil
	settingEnvOnce = sync.Once{}
	settingEnv = nil
}
