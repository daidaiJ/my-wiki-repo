package registry

// 拷贝模式：项目侧知识目录保持真目录（正文归项目 git 管），
// 知识库侧 projects/<项目>/ 存放其增量合并拷贝。
// 同步为合并单向：项目 → 知识库只拷贝新增/修改的文件；知识库侧的修改
// （Obsidian 校对）永不覆盖；永不删文件——项目侧删除的内容在知识库保留。

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
)

// ensureCopyMode 把一条知识路径收敛到拷贝模式布局：项目侧与知识库侧均为真目录。
// 项目侧缺失且 provision（prepare hook）时只预置空知识库目录，等项目侧出现后由 check 补拷。
// 项目侧或知识库侧任一存在链接（残留链接模式布局）时拒绝动手，防止错误覆盖搬迁。
// 返回是否发生了拷贝或新建。
func ensureCopyMode(store, projPath string, provision bool) (bool, error) {
	if isLink(projPath) {
		return false, fmt.Errorf("项目侧 %s 是链接（疑似链接模式布局），拷贝模式拒绝覆盖，请先手动处理", projPath)
	}
	if isLink(store) {
		return false, fmt.Errorf("知识库侧 %s 是链接，与拷贝模式不符，请先手动处理", store)
	}
	if !isRealDir(projPath) {
		if !provision {
			return false, fmt.Errorf("项目知识目录不存在: %s", projPath)
		}
		if isRealDir(store) {
			return false, nil
		}
		if err := os.MkdirAll(store, 0o755); err != nil {
			return false, err
		}
		return true, nil
	}
	if !isRealDir(store) {
		// 首次接入：全量拷贝进知识库，项目侧不动
		if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
			return false, err
		}
		if err := copyTree(projPath, store); err != nil {
			return false, fmt.Errorf("首次拷贝到知识库失败: %w", err)
		}
		return true, nil
	}
	return mergeCopy(projPath, store)
}

// mergeCopy 增量合并：把 src 中新增/修改的文件拷到 dst，dst 独有内容一概不动。
// 判定与 rsync -u 一致：目标缺失或 src mtime 更晚才拷贝（新者胜）。
// 拷贝后的目标 mtime 为当前时间，天然避免未变更文件被反复重拷；
// 知识库侧更晚的修改（Obsidian 校对）因此不会被项目侧覆盖。
// 局限：项目侧被外部工具改写且 mtime 被设为过去（如解压旧归档）时不会被发现。
func mergeCopy(src, dst string) (bool, error) {
	changed := false
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if skipMigrateName(name) {
			if d.IsDir() || isLink(path) {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if isLink(path) {
			if d.IsDir() || (d.Type()&os.ModeSymlink != 0 && cli.DirExists(path)) {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		out := filepath.Join(dst, rel)
		need, err := copyNeeded(path, out)
		if err != nil {
			return err
		}
		if !need {
			return nil
		}
		if err := copyFile(path, out); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// copyNeeded 目标缺失或源 mtime 更晚（新者胜）时才需要拷贝。
// 不比 size：size 差异但知识库侧更新的场景（Obsidian 校对）必须保留。
func copyNeeded(srcPath, dstPath string) (bool, error) {
	si, err := os.Stat(srcPath)
	if err != nil {
		return false, err
	}
	di, err := os.Stat(dstPath)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return si.ModTime().After(di.ModTime()), nil
}
