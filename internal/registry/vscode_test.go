package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureVSCodeWorkspace(t *testing.T) {
	wiki := t.TempDir()

	// 首次：生成全部配置
	created, err := ensureVSCodeWorkspace(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != len(vscodeBootstrapFiles) {
		t.Fatalf("首次应生成 %d 个文件, got %v", len(vscodeBootstrapFiles), created)
	}
	for _, f := range vscodeBootstrapFiles {
		data, err := os.ReadFile(filepath.Join(wiki, f.rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != f.content {
			t.Errorf("%s 内容不符:\n%s", f.rel, data)
		}
	}

	// 二次：已存在则不动，返回空
	created, err = ensureVSCodeWorkspace(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 {
		t.Errorf("二次调用不应再写盘, got %v", created)
	}

	// 用户改过的文件不被覆盖
	p := filepath.Join(wiki, vscodeBootstrapFiles[0].rel)
	if err := os.WriteFile(p, []byte("{\"custom\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureVSCodeWorkspace(wiki); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"custom\":true}\n" {
		t.Errorf("用户改动被覆盖:\n%s", data)
	}
}
