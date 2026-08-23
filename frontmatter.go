package main

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"
)

// FrontMatter 是 blog list / new 实际关心的 front matter 字段子集。
type FrontMatter struct {
	Title      string
	Slug       string
	Categories []string
	Tags       []string
}

// NewPostMeta 是创建一篇文章所需的元信息，其余字段全部由模板确定性生成。
type NewPostMeta struct {
	Title      string
	Slug       string
	Categories []string
	Tags       []string
	Now        time.Time
}

// generateFrontMatter 按博客既有文章的模板逐字段生成 front matter：
// musicid 固定、image 用 slug 哈希派生 picsum seed，date/lastmod 取当前时间。
func generateFrontMatter(m NewPostMeta) string {
	ts := m.Now.Format("2006-01-02T15:04:05-07:00")
	seed := fmt.Sprintf("%x", md5.Sum([]byte(m.Slug)))[4:12]
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", yamlQuote(m.Title))
	fmt.Fprintf(&b, "slug: %s\n", m.Slug)
	b.WriteString("description: \"\"\n")
	fmt.Fprintf(&b, "date: %s\n", ts)
	fmt.Fprintf(&b, "lastmod: %s\n", ts)
	b.WriteString("draft: false\n")
	b.WriteString("toc: true\n")
	b.WriteString("hidden: false\n")
	b.WriteString("weight: false\n")
	b.WriteString("musicid: 5264842\n")
	b.WriteString("qqmusic: \n")
	b.WriteString("categories:\n")
	for _, c := range m.Categories {
		fmt.Fprintf(&b, "    - %s\n", c)
	}
	b.WriteString("tags :\n")
	for _, t := range m.Tags {
		fmt.Fprintf(&b, "    - %s\n", t)
	}
	fmt.Fprintf(&b, "image: https://picsum.photos/seed/%s/800/600\n", seed)
	b.WriteString("---")
	return b.String()
}

// yamlQuote 生成 YAML 双引号标量：只转义反斜杠和双引号，中文等非 ASCII 保持原样。
func yamlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
