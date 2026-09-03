package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultModeAndInjectFileConfig(t *testing.T) {
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_DEFAULT_MODE", "")
	t.Setenv("WIKI_INJECT_FILE", "")

	// 缺省：link 模式 + ~/.qwen/QWEN.md
	if got := DefaultMode(); got != "link" {
		t.Errorf("默认 defaultMode = %q", got)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("无 home 目录环境")
	}
	if got, _ := InjectFile(); got != filepath.Join(home, ".qwen", "QWEN.md") {
		t.Errorf("默认 injectFile = %q", got)
	}

	// config.json
	cfg := &wikiConfig{DefaultMode: "copy", InjectFile: filepath.Join(wiki, "AGENTS.md")}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := DefaultMode(); got != "copy" {
		t.Errorf("config defaultMode = %q", got)
	}
	if got, _ := InjectFile(); got != filepath.Join(wiki, "AGENTS.md") {
		t.Errorf("config injectFile = %q", got)
	}

	// env 覆盖 config
	t.Setenv("WIKI_DEFAULT_MODE", "link")
	t.Setenv("WIKI_INJECT_FILE", `X:\notes\CLAUDE.md`)
	if got := DefaultMode(); got != "link" {
		t.Errorf("env defaultMode = %q", got)
	}
	if got, _ := InjectFile(); got != `X:\notes\CLAUDE.md` {
		t.Errorf("env injectFile = %q", got)
	}
}
