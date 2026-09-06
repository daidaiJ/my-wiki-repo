package registry

import (
	"os"
	"path/filepath"
)

// VSCode 工作区引导配置：projects 目录下点开 .md 直接进内置预览（无需装插件）。
// JSONC 格式，VSCode 原生支持注释。
const vscodeSettingsJSON = `{
  // 点击 .md 文件直接打开内置 Markdown 预览（渲染视图），而不是源码
  "workbench.editorAssociations": {
    "*.md": "vscode.markdown.preview.editor"
  },
  // 在预览页面双击即可切换回源码编辑
  "markdown.preview.doubleClickToSwitchToEditor": true,
  // 单个换行符也渲染为换行（wiki 文档常见习惯）
  "markdown.preview.breaks": true,
  // 预览中点击文档内相对链接时在预览中打开
  "markdown.links.openWorkspaceLinks": true
}
`

const vscodeExtensionsJSON = `{
  // 打开本工作区时自动推荐安装（Mermaid 图表渲染、Markdown 编辑增强）
  "recommendations": [
    "bierner.markdown-mermaid",
    "yzhang.markdown-all-in-one"
  ]
}
`

// vscodeBootstrapFiles 是要补齐的配置文件（相对 wiki 根，顺序固定便于稳定输出）。
var vscodeBootstrapFiles = []struct {
	rel     string
	content string
}{
	{filepath.Join(ProjectsRootName, ".vscode", "settings.json"), vscodeSettingsJSON},
	{filepath.Join(ProjectsRootName, ".vscode", "extensions.json"), vscodeExtensionsJSON},
}

// ensureVSCodeWorkspace 幂等补齐 projects/.vscode 下的编辑器预览配置。
// 仅在文件缺失时写入，绝不覆盖用户已有内容。返回本次创建的相对路径列表。
func ensureVSCodeWorkspace(root string) ([]string, error) {
	var created []string
	for _, f := range vscodeBootstrapFiles {
		path := filepath.Join(root, f.rel)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return created, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return created, err
		}
		if err := os.WriteFile(path, []byte(f.content), 0o644); err != nil {
			return created, err
		}
		created = append(created, f.rel)
	}
	return created, nil
}
