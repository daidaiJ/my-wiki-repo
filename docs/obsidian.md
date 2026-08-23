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

3. `wiki init <项目>` 接入项目——注册表 `index.md`、`config.json`、`projects/` 链接全部落在仓库目录里，Obsidian 里立即可见
4. 验证：`wiki ls` 项目在册，`obsidian vault=<仓库名> folders` 索引完整

## 路线二：顺序反了，迁移现有持久化目录

wiki CLI 数据已经存在（比如在旧位置），迁移进 Obsidian 仓库分四步：

1. **迁数据**：`index.md`（注册表）、`config.json`、`blog.json`、`projects/` 整个目录移进仓库
2. **显式指定 WIKI_ROOT**：wiki 根靠 `index.md` 标记解析（环境变量 > exe 目录 > 当前目录），只搬文件不设环境变量，钩子会静默失效、`wiki check` 空转：

```bash
# Windows
setx WIKI_ROOT "D:\path\to\vault"
# macOS / Linux
export WIKI_ROOT=/path/to/vault
```

钩子不依赖终端环境，内联设置：

```bash
# Windows（cmd）
cmd /c "set WIKI_ROOT=D:\path\to\vault&& wiki check"
# macOS / Linux
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
- `obsidian vault=<仓库名> folders/files`：符号链接内容全部索引（Obsidian 原生支持 symlink，约束：目标与仓库根不相交、无循环）
- 最终结构：

```
<vault>/
├── .obsidian/
├── index.md                  ← 注册表（标记）
├── config.json / blog.json
└── projects/                 ← 各项目知识目录链接
    ├── project-a/wiki -> /path/to/project-a/wiki
    ├── project-b/wiki -> /path/to/project-b/wiki
    └── ...
```

> 两条路线殊途同归：Obsidian 仓库根 = wiki 根，一个目录两用。推荐路线一，但路线二也不复杂——迁数据、设 WIKI_ROOT、注册、验证四步，十分钟收工。