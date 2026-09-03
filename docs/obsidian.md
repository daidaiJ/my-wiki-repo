# Obsidian 仓库接入

> 把 wiki 根接进 Obsidian 当仓库有两条路：先建仓库再配 wiki CLI（推荐），或者顺序反了之后迁移。殊途同归：Obsidian 仓库根 = wiki 根，一个目录两用。

## 路线一：先建仓库，再配置 wiki CLI

正确顺序是先有仓库，再让 wiki CLI 的持久化目录落进仓库：

1. 建一个空的 Obsidian 仓库，目录名避开 `wiki`——和 `knowledgeDirs` 的类型名重合，自动发现时可能把仓库目录自己误认成项目知识目录
2. 把 wiki CLI 持久化目录配置到仓库（用户级环境变量，新终端生效）：

```bash
# Windows
setx WIKI_ROOT "D:\path\to\vault"
# macOS / Linux（写入 ~/.zshrc / ~/.bashrc）
export WIKI_ROOT=/path/to/vault
```

3. `wiki init <项目>` 接入项目——注册表 `index.md`、`config.json`、`projects/` 正文全部落在仓库目录里，Obsidian 里立即可见
4. 注册 **Start + Stop** hook（`wiki prepare` + `wiki check`），换机器后窗口链接自动重建
5. 验证：`wiki ls` 项目在册，`obsidian vault=<仓库名> folders` 索引完整

## 路线二：顺序反了，迁移现有持久化目录

wiki CLI 数据已经存在（比如在旧位置），迁移进 Obsidian 仓库分四步：

1. **迁数据**：`index.md`（注册表）、`config.json`、`blog.json`、`projects/` 整个目录移进仓库
2. **显式指定 WIKI_ROOT**：wiki 根靠 `index.md` 标记解析（环境变量 > exe 目录 > 当前目录），只搬文件不设环境变量，hook 会静默失效：

```bash
# Windows
setx WIKI_ROOT "D:\path\to\vault"
# macOS / Linux
export WIKI_ROOT=/path/to/vault
```

hook 内联设置：

```bash
# Windows（cmd）
cmd /c "set WIKI_ROOT=D:\path\to\vault&& wiki.exe prepare"
cmd /c "set WIKI_ROOT=D:\path\to\vault&& wiki.exe check"
# macOS / Linux
WIKI_ROOT=/path/to/vault wiki prepare
WIKI_ROOT=/path/to/vault wiki check
```

3. **注册仓库**：UI 里 Open folder as vault，或直接改注册表：

```json
// %APPDATA%\obsidian\obsidian.json（Windows）
{"vaults":{"<16位hex id>":{"path":"D:\\path\\to\\vault","ts":<毫秒时间戳>,"open":true}},"cli":true}
```

> `obsidian://open?path=` 只能解析已注册仓库里的文件，不能注册新仓库——别在这上面浪费时间。

4. **验证清单**：

- `wiki ls`：项目全部在册
- `obsidian vault=<仓库名> folders/files`：`projects/` 下正文全部索引
- 最终结构：

```
<vault>/
├── .obsidian/
├── index.md                  ← 注册表（标记）
├── config.json / blog.json
└── projects/                 ← 各项目知识正文（真文件，可 git）
    ├── project-a/wiki/
    │   └── note.md
    ├── project-b/issues/
    │   └── ...
    └── ...
```

项目侧（不在 Obsidian vault 内）：

```
/path/to/project-a/
├── wiki/                     ← 窗口链接 → <vault>/projects/project-a/wiki
└── .gitignore                ← wiki 维护的知识目录 ignore 条目
```

> 两条路线殊途同归：Obsidian 仓库根 = wiki 根，一个目录两用。方案 C 下 Obsidian 直接读 `projects/` 真文件，比旧版符号链接聚合更简单、git 更友好。
