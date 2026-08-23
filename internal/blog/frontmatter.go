package blog

import (
	"strings"
)

// FrontMatter 是 blog list / new 实际关心的 front matter 字段子集。
type FrontMatter struct {
	Title      string
	Slug       string
	Categories []string
	Tags       []string
}

// NewPostMeta 是创建一篇文章需要填充的四字段元信息。
type NewPostMeta struct {
	Title      string
	Slug       string
	Categories []string
	Tags       []string
}

// fillFrontMatter 在 `hugo new` 按主题 archetype 生成的模板上外科手术式地
// 填入四个字段（title/slug/categories/tags），其余行（musicid/image/date 等
// 主题自有字段）保持字节不动；缺失的键补在结束分隔符前。
// 返回完整的文档内容（front matter + 原样保留的正文占位），调用方再追加正文。
func fillFrontMatter(archetype string, m NewPostMeta) string {
	lines := strings.Split(strings.ReplaceAll(archetype, "\r\n", "\n"), "\n")
	titleDone, slugDone, catsDone, tagsDone := false, false, false, false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case !titleDone && matchKey(line, "title"):
			lines[i] = "title: " + yamlQuote(m.Title)
			titleDone = true
		case !slugDone && matchKey(line, "slug"):
			lines[i] = "slug: " + m.Slug
			slugDone = true
		case !catsDone && matchKey(line, "categories"):
			var block []string
			block = append(block, "categories:")
			for _, c := range m.Categories {
				block = append(block, "    - "+c)
			}
			lines = replaceListEntry(lines, i, block)
			catsDone = true
		case !tagsDone && matchKey(line, "tags"):
			var block []string
			block = append(block, "tags:")
			for _, tg := range m.Tags {
				block = append(block, "    - "+tg)
			}
			lines = replaceListEntry(lines, i, block)
			tagsDone = true
		}
	}

	// archetype 没生成的键：补在结束 --- 之前
	var missing []string
	if !titleDone {
		missing = append(missing, "title: "+yamlQuote(m.Title))
	}
	if !slugDone {
		missing = append(missing, "slug: "+m.Slug)
	}
	if !catsDone {
		missing = append(missing, "categories:")
		for _, c := range m.Categories {
			missing = append(missing, "    - "+c)
		}
	}
	if !tagsDone {
		missing = append(missing, "tags:")
		for _, tg := range m.Tags {
			missing = append(missing, "    - "+tg)
		}
	}
	if len(missing) > 0 {
		if end := closingFenceIndex(lines); end >= 0 {
			merged := append([]string{}, lines[:end]...)
			merged = append(merged, missing...)
			merged = append(merged, lines[end:]...)
			lines = merged
		}
	}
	return strings.Join(lines, "\n")
}

// matchKey 判断一行是否是 `key:` 形式的 YAML 键（容忍冒号前后空格，
// 兼容既有文章 `tags :` 的写法），不匹配嵌套列表项。
func matchKey(line, key string) bool {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, key) {
		return false
	}
	rest := t[len(key):]
	return strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, " :")
}

// replaceListEntry 把键所在行连同其后续缩进列表项替换为新块（archetype 常见的
// `categories: [""]` 流式写法或空值都只占键这一行，块式写法占多行）。
func replaceListEntry(lines []string, idx int, block []string) []string {
	end := idx + 1
	for end < len(lines) {
		t := lines[end]
		if strings.HasPrefix(t, " ") || strings.HasPrefix(t, "\t") || strings.HasPrefix(t, "- ") {
			end++
		} else {
			break
		}
	}
	merged := append([]string{}, lines[:idx]...)
	merged = append(merged, block...)
	merged = append(merged, lines[end:]...)
	return merged
}

func closingFenceIndex(lines []string) int {
	for i, l := range lines {
		if i > 0 && strings.TrimRight(l, " ") == "---" {
			return i
		}
	}
	return -1
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
