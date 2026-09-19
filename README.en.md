> [中文](README.md)

# my-wiki

[![CI](https://github.com/daidaiJ/my-wiki-repo/actions/workflows/ci.yml/badge.svg)](https://github.com/daidaiJ/my-wiki-repo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/daidaiJ/my-wiki-repo)](https://github.com/daidaiJ/my-wiki-repo/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/daidaiJ/my-wiki-repo)](go.mod)
[![License](https://img.shields.io/github/license/daidaiJ/my-wiki-repo)](LICENSE)

**A CLI that gathers research notes scattered across repos into one place.** Canonical notes live at the wiki root (git-friendly). The project side is a window symlink by default, or a real directory plus incremental copies (`--mode copy`). A session-end hook keeps it maintained, and optionally automates the mechanical steps of publishing a Hugo blog.

## Architecture

Four layers: the **client** (Claude / ZCode / Qwen) hits the **control plane** (open-source `wiki` CLI) via a session-end hook. Agents still write `project/wiki/`; notes land in the **data plane** `WIKI_ROOT/projects/` through the window. Exits are Obsidian review, git sync, and Hugo publish.

![my-wiki architecture: control flow vs data plane](docs/architecture.png)

| Layer | Role |
|---|---|
| **Client** | Claude Code / ZCode / Qwen Code / other agents |
| **Control plane** | `wiki` CLI: init / check / grep / blog |
| **Data plane** | `WIKI_ROOT`: registry `index.md`, `projects/` notes, `config.json` |
| **Exits** | Obsidian review · git sync · Hugo publish |

## 📖 Docs

| Reader | Doc | Notes |
|---|---|---|
| 🤖 AI Agent | [docs/AGENT_GUIDE.en.md](docs/AGENT_GUIDE.en.md) | Token-efficient: commands, hook, config, publish hard rules |
| 👤 Human users | [docs/HUMAN_GUIDE.en.md](docs/HUMAN_GUIDE.en.md) | Readable: install, hook, daily use, config cheat sheet, blog |
| 🔍 Design | [docs/design.en.md](docs/design.en.md) | Hook-driven design, data vs control, link / copy |
| 🔄 Workflow | [docs/workflow.en.md](docs/workflow.en.md) | Agent notes → Obsidian review → Hugo publish |
| 📓 Obsidian | [docs/obsidian.en.md](docs/obsidian.en.md) | New vault first / migrate existing data |

Docs are Chinese by default; the same-name `.en.md` files are the English versions.

## ✨ Highlights

| Point | One line |
|---|---|
| **Single hook** | Session-end `check` maintains windows, absorbs newly added knowledge dirs, and auto-inverts unregistered projects — agents never notice |
| **Stable paths** | Agents still write `wiki/note.md`; they never need the wiki absolute path |
| **Cross-project grep** | `wiki grep <pattern>` searches every enrolled project |
| **VSCode preview** | `projects/` gets `.vscode` config so `.md` opens as rendered preview |
| **Tool / data split** | Open-source CLI + local `WIKI_ROOT`; personal config never hits a project remote |

## Why it exists

If you often ask Claude Code / Codex / Qwen Code to research code and write up designs, you probably hit the same problem: the artifacts (wiki notes, issue write-ups, war stories) scatter across a dozen repos, unindexed, in inconsistent shapes. Forking a wiki repo per project is too heavy.

my-wiki’s approach: **notes in the wiki, windows on the project**. Agents still write `project/wiki/`; the tool stores content in `WIKI_ROOT/projects/` (git-friendly); the project side is just a window. The registry `index.md` is local, so personal config never lands on a project remote. `wiki grep` searches across projects anytime; on a new machine clone the wiki and `wiki prepare` rebuilds the windows.

**A good fit:** parallel research across repos; coding agents that should leave notes a session-end hook can maintain; a personal Hugo blog where you are tired of hand-writing front matter.

**Not a fit:** a shared team wiki (the registry is single-machine); out-of-the-box remote sync — put a private git remote on `projects/`, or `wiki bundle` for an on-demand archive.

## Where the notes live

The live layout is **inverted storage**: canonical files in the wiki, windows on the project side. The other two layouts are not choices — **forward aggregation** is retired (auto-migrated when found), **on-demand archive** is only a backup exit.

![Three layouts: forward aggregation, inverted storage, on-demand archive](docs/layouts.png)

Docs only describe inverted storage because it is the only live layout. Forward aggregation migrates away; on-demand archive is not a third daily mode, just `wiki bundle` making a takeaway copy of the canonical tree. **link / copy** are how the project side *sees* inverted storage, not another layout. Comparison: [design doc](docs/design.en.md).

## 🚀 Quick start

```bash
git clone https://github.com/daidaiJ/my-wiki-repo.git
cd my-wiki-repo && go build -o wiki ./cmd/wiki

# Enroll a project (auto-discovers wiki/ and issues/)
./wiki init /path/to/some-project --intro "one-line intro"

# Cross-project search
./wiki ls                              # registered projects
./wiki grep "controller reconcile"     # content search
./wiki cat some-project/wiki/xxx.md    # read a hit
```

See the two guides above for configuration, hooks, and daily commands.

## License

MIT License, see [LICENSE](LICENSE).
