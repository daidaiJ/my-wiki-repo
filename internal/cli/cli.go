// Package cli 提供 wiki 各命令共享的小工具：宽容的 flag 解析、
// 字符串/路径助手与 JSON 序列化，不含业务语义。
package cli

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
)

// ParseWithPositionals 宽松解析：允许位置参数出现在旗标前（如 `init <目录> --paths wiki`），
// 内部重排为「旗标在前、位置参数在后」再交给标准 FlagSet。
func ParseWithPositionals(fs *flag.FlagSet, args []string) error {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if strings.Contains(a, "=") {
				continue
			}
			if f := fs.Lookup(strings.TrimLeft(a, "-")); f != nil {
				if bv, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bv.IsBoolFlag() {
					if i+1 < len(args) { // 该旗标需要显式值，吞掉下一个 token
						i++
						flags = append(flags, args[i])
					}
				}
			}
			continue
		}
		pos = append(pos, a)
	}
	return fs.Parse(append(flags, pos...))
}

// SplitCSV 按逗号切分并去空去重（保留顺序）。
func SplitCSV(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" && !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

// Pick 新值非空取新值，否则保留旧值。
func Pick(newVal, oldVal string) string {
	if newVal != "" {
		return newVal
	}
	return oldVal
}

// Ternary 三目表达式助手。
func Ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// UnderOrEqual 判断 path 是否等于 root 或位于其下（大小写不敏感，适配 Windows）。
func UnderOrEqual(path, root string) bool {
	p := strings.ToLower(filepath.Clean(path))
	r := strings.ToLower(filepath.Clean(root))
	return p == r || strings.HasPrefix(p+string(filepath.Separator), r+string(filepath.Separator))
}

// SamePath 比较两个路径是否指向同一位置（大小写不敏感）。
func SamePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// DirExists 判断目录是否存在。
func DirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// JSONIndent 序列化为缩进 JSON（小工具场景假定不会失败）。
func JSONIndent(v any) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return data
}
