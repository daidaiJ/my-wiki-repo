package blog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
)

// 博客发布记录：wiki 侧懒维护的一份「已发布文档四字段」数据（blog.json）。
//   - 首次 blog list 时扫描 Hugo 文章目录引导生成
//   - 之后按文件名增量对账：只解析新增文件、移除已删除文件，不重扫全量
//   - blog publish 成功后把该文章四字段写入记录
//
// blog list 只聚合 categories/tags 两字段；title/slug 重复罕见，
// 在 blog new（apply 时）直接报错即可。

type BlogRecEntry struct {
	File       string   `json:"file"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Categories []string `json:"categories"`
	Tags       []string `json:"tags"`
}

type BlogRecord struct {
	Posts []BlogRecEntry `json:"posts"`
}

func recordPath(root string) string { return filepath.Join(root, "blog.json") }

func loadBlogRecord(root string) (*BlogRecord, error) {
	rec := &BlogRecord{}
	data, err := os.ReadFile(recordPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return rec, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, rec); err != nil {
		return nil, fmt.Errorf("blog.json 解析失败: %w", err)
	}
	return rec, nil
}

func (r *BlogRecord) save(root string) error {
	sort.Slice(r.Posts, func(i, j int) bool { return r.Posts[i].File < r.Posts[j].File })
	return os.WriteFile(recordPath(root), cli.JSONIndent(r), 0o644)
}

func (r *BlogRecord) upsert(e BlogRecEntry) {
	for i := range r.Posts {
		if r.Posts[i].File == e.File {
			r.Posts[i] = e
			return
		}
	}
	r.Posts = append(r.Posts, e)
}

// reconcileRecord 按文件名与 Hugo 文章目录增量对账（lazy：已记录的文件不重新解析）。
func reconcileRecord(root, postDir string) (*BlogRecord, error) {
	rec, err := loadBlogRecord(root)
	if err != nil {
		return nil, err
	}
	entries, dirErr := os.ReadDir(postDir)
	if dirErr != nil {
		if len(rec.Posts) > 0 {
			// 目录暂不可用但已有记录：用旧记录兜底
			fmt.Fprintf(os.Stderr, "wiki: 文章目录 %s 暂不可读，使用本地记录（%d 篇）\n", postDir, len(rec.Posts))
			return rec, nil
		}
		return nil, fmt.Errorf("文章目录不可用: %s（wiki config set blogRepo 配置）: %w", postDir, dirErr)
	}

	known := map[string]bool{}
	for _, e := range rec.Posts {
		known[e.File] = true
	}
	onDisk := map[string]bool{}
	for _, en := range entries {
		if en.IsDir() || !strings.HasSuffix(en.Name(), ".md") {
			continue
		}
		onDisk[en.Name()] = true
		if known[en.Name()] {
			continue
		}
		entry, err := parsePostFile(filepath.Join(postDir, en.Name()))
		if err != nil {
			continue // 坏 front matter 不阻塞
		}
		rec.Posts = append(rec.Posts, entry)
	}
	// 移除已消失的文件
	kept := rec.Posts[:0]
	for _, e := range rec.Posts {
		if onDisk[e.File] {
			kept = append(kept, e)
		}
	}
	rec.Posts = kept
	if err := rec.save(root); err != nil {
		return nil, err
	}
	return rec, nil
}

// parsePostFile 从文章文件解析四字段记录。
func parsePostFile(path string) (BlogRecEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BlogRecEntry{}, err
	}
	entry := BlogRecEntry{File: filepath.Base(path)}
	fm, err := parseFrontMatter(string(data))
	if err != nil {
		return entry, nil // 坏 front matter：只记文件名
	}
	entry.Title, entry.Slug, entry.Categories, entry.Tags = fm.Title, fm.Slug, fm.Categories, fm.Tags
	return entry, nil
}

// aggregate 统计某类字段的词频（categories 或 tags）。
func aggregate(rec *BlogRecord, pick func(BlogRecEntry) []string) map[string]int {
	m := map[string]int{}
	for _, p := range rec.Posts {
		for _, v := range pick(p) {
			m[v]++
		}
	}
	return m
}

func sortedByCount(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if m[out[i]] != m[out[j]] {
			return m[out[i]] > m[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}
