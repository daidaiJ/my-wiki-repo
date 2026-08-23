package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectGuideLifecycle(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "QWEN.md")

	// 1. 文件不存在 → 创建
	action, err := injectGuide(target)
	if err != nil || action != "created" {
		t.Fatalf("首次注入: action=%s err=%v", action, err)
	}
	created, _ := os.ReadFile(target)
	if !strings.Contains(string(created), guideStartMarker) {
		t.Error("创建的文件应含标记段")
	}

	// 2. 幂等：重跑 unchanged，不写盘
	before, _ := os.ReadFile(target)
	action, err = injectGuide(target)
	if err != nil || action != "unchanged" {
		t.Fatalf("重复注入: action=%s err=%v", action, err)
	}
	after, _ := os.ReadFile(target)
	if string(before) != string(after) {
		t.Error("unchanged 时不应写盘")
	}

	// 3. 摘除 → 恢复为空壳
	action, err = removeGuide(target)
	if err != nil || action != "removed" {
		t.Fatalf("摘除: action=%s err=%v", action, err)
	}
	got, _ := os.ReadFile(target)
	if len(strings.TrimSpace(string(got))) != 0 {
		t.Errorf("摘除后应只剩空壳, got %q", got)
	}
}

func TestInjectGuideIntoExistingContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "QWEN.md")
	existing := "# Qwen Code 个人配置\n\n## 既有章节\n\n- 既有内容，不能动\n"
	os.WriteFile(target, []byte(existing), 0o644)

	// 追加：与既有内容空行分隔
	action, err := injectGuide(target)
	if err != nil || action != "appended" {
		t.Fatalf("追加: action=%s err=%v", action, err)
	}
	got, _ := os.ReadFile(target)
	s := string(got)
	if !strings.HasPrefix(s, existing) {
		t.Error("追加不应改动既有内容")
	}
	if !strings.Contains(s, "- 既有内容，不能动") || !strings.Contains(s, guideStartMarker) {
		t.Errorf("追加结果异常:\n%s", s)
	}

	// 模拟旧版本引导段（标记相同、内容不同）→ 原位替换
	oldSection := guideStartMarker + "\n## 旧版 wiki 引导\n旧内容\n" + guideEndMarker
	os.WriteFile(target, []byte(existing+"\n"+oldSection+"\n"), 0o644)
	action, err = injectGuide(target)
	if err != nil || action != "replaced" {
		t.Fatalf("替换: action=%s err=%v", action, err)
	}
	got, _ = os.ReadFile(target)
	s = string(got)
	if strings.Contains(s, "旧版 wiki 引导") {
		t.Error("旧内容应被替换掉")
	}
	if !strings.Contains(s, "- 既有内容，不能动") || strings.Count(s, guideStartMarker) != 1 {
		t.Errorf("替换后异常:\n%s", s)
	}
}

func TestRemoveGuideCleanup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "QWEN.md")
	content := "# 配置\n\n" + guideSection() + "\n\n# 尾部章节\n"
	os.WriteFile(target, []byte(content), 0o644)

	action, err := removeGuide(target)
	if err != nil || action != "removed" {
		t.Fatalf("摘除: action=%s err=%v", action, err)
	}
	got, _ := os.ReadFile(target)
	s := string(got)
	if strings.Contains(s, guideStartMarker) {
		t.Error("标记段未摘除")
	}
	if strings.Contains(s, "\n\n\n") {
		t.Errorf("应清理连续空行:\n%q", s)
	}
	if !strings.Contains(s, "# 尾部章节") {
		t.Error("摘除不应影响其他章节")
	}
}
