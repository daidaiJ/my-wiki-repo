package main

import (
	"strings"
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 8, 23, 10, 30, 0, 0, time.FixedZone("CST", 8*3600))
}

func TestGenerateFrontMatterGolden(t *testing.T) {
	got := generateFrontMatter(NewPostMeta{
		Title:      "Google ax + substrate：智能体运行时调度架构分析",
		Slug:       "google-ax-agent-runtime",
		Categories: []string{"技术笔记", "AI"},
		Tags:       []string{"智能体", "kubernetes"},
		Now:        fixedTime(),
	})
	want := `---
title: "Google ax + substrate：智能体运行时调度架构分析"
slug: google-ax-agent-runtime
description: ""
date: 2026-08-23T10:30:00+08:00
lastmod: 2026-08-23T10:30:00+08:00
draft: false
toc: true
hidden: false
weight: false
musicid: 5264842
qqmusic: 
categories:
    - 技术笔记
    - AI
tags :
    - 智能体
    - kubernetes
image: https://picsum.photos/seed/3b36cb88/800/600
---`
	if got != want {
		t.Errorf("front matter 与预期不一致:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// axPostFM 是线上真实文章的 front matter（含 `tags :` 冒号前空格的既有写法）。
const axPostFM = `---
title: "Google ax + substrate：智能体运行时调度架构分析"
slug: google-ax-agent-runtime
description: ""
date: 2026-06-07T09:53:32+08:00
lastmod: 2026-06-07T09:53:32+08:00
draft: false
toc: true
hidden: false
weight: false
musicid: 5264842
qqmusic: 
categories:
    - 技术笔记
    - AI
tags :
    - 智能体
    - kubernetes
image: https://picsum.photos/seed/40aa71ea/800/600
---

# 正文标题

正文内容。
`

func TestParseFrontMatterRealPost(t *testing.T) {
	fm, err := parseFrontMatter(axPostFM)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Title != "Google ax + substrate：智能体运行时调度架构分析" {
		t.Errorf("title = %q", fm.Title)
	}
	if fm.Slug != "google-ax-agent-runtime" {
		t.Errorf("slug = %q", fm.Slug)
	}
	if strings.Join(fm.Categories, ",") != "技术笔记,AI" {
		t.Errorf("categories = %v", fm.Categories)
	}
	if strings.Join(fm.Tags, ",") != "智能体,kubernetes" {
		t.Errorf("tags = %v", fm.Tags)
	}
}

func TestParseFrontMatterFlowStyle(t *testing.T) {
	content := "---\ntitle: \"x\"\ncategories: [\"a\", \"b\"]\ntags: [\"golang\"]\nslug: x\n---\nbody"
	fm, err := parseFrontMatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fm.Categories, ",") != "a,b" {
		t.Errorf("categories = %v", fm.Categories)
	}
	if strings.Join(fm.Tags, ",") != "golang" {
		t.Errorf("tags = %v", fm.Tags)
	}
}

func TestParseFrontMatterErrors(t *testing.T) {
	if _, err := parseFrontMatter("no front matter"); err == nil {
		t.Error("缺开头 --- 应报错")
	}
	if _, err := parseFrontMatter("---\ntitle: \"x\"\n"); err == nil {
		t.Error("缺结束 --- 应报错")
	}
}

func TestYamlQuote(t *testing.T) {
	cases := map[string]string{
		"普通中文":      `"普通中文"`,
		`含"引号"`:     `"含\"引号\""`,
		`含\反斜杠`:     `"含\\反斜杠"`,
		`Agent 运行时`: `"Agent 运行时"`,
	}
	for in, want := range cases {
		if got := yamlQuote(in); got != want {
			t.Errorf("yamlQuote(%q) = %s, want %s", in, got, want)
		}
	}
}
