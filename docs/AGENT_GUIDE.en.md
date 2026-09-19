> [中文版](AGENT_GUIDE.md)

# my-wiki — Agent Guide

Cross-project knowledge-base CLI (Go). Canonical notes live in `WIKI_ROOT/projects/`; project-side `wiki/` / `issues/` are windows. Single hook: session-end `wiki check`. **Do not hand-edit `index.md` / `blog.json`.**

> Human-readable: [HUMAN_GUIDE.en.md](HUMAN_GUIDE.en.md). Workspace contract: [AGENTS.md](../AGENTS.md). Skill: [SKILL.md](../SKILL.md).

## Storage

Current **scheme C (inverted)**: real files in the wiki, windows on the project side. Scheme A (forward links) is auto-migrated to C. Scheme B = `wiki bundle` snapshot, not a daily layout.

| mode | Project side | Sync |
|---|---|---|
| `link` (default) | Window → wiki; writes `.gitignore` | None |
| `copy` | Real dirs, owned by project git | Project→wiki incremental merge; newer mtime wins; **never deletes** |

`--mode` applies to new enrollments only. Switch: `unlink` then `init`. No flat enroll of `.`. Do not enroll upstream `docs/`. Each enrolled dir needs a `README.md` index.

Path shape: `project/link/relative`. Agents still write `wiki/note.md`; they never need the wiki absolute path.

## Flows (independent)

| Flow | When | Command |
|---|---|---|
| Repair (optional) | Anytime | `wiki prepare` — windows for registered projects only; no new enrollments |
| Sync | Session-end hook | `wiki check` |
| Metadata | Anytime | `wiki init --intro/--summary` |
| Publish | When the user explicitly asks | `wiki blog new` → `wiki blog publish` |

Knowledge pipeline: ① agent writes project `wiki/` → ② **human** reviews in Obsidian (do not skip) → ③ `blog new` / `publish`. Obsidian root = wiki root; see [obsidian.en.md](obsidian.en.md).

## Commands

```
wiki init [dir] --paths <dirs> [--mode copy|link] [--intro ...] [--summary ...]
wiki list
wiki sync [--fix]
wiki unlink <name> [--purge]            # notes kept by default
wiki prepare                            # optional, not a hook
wiki check                              # the only hook; empty stdout, logs on stderr
wiki bundle [--dir <dir>] [--archive zip|tgz]
wiki ls [project[/subpath]]
wiki tree [<project>] [--depth N]
wiki grep <pattern> [<subpath>] [--fixed]   # output feeds wiki cat
wiki cat <project/.../file>
wiki config / wiki config set <key> <value>
wiki inject [--file <instruction-file>] [--remove]
```

Windows: symlink, junction fallback without permission.

## Hook

Register session-end `wiki check` only. Empty stdout; failures do not block. Registered projects: newly added knowledge dirs are absorbed automatically (copy body into the vault first, then invert into a window link; registered paths never shrink). Unregistered + existing knowledge dirs → auto-invert (after a write, never empty shells). Same-name registry conflicts are rejected.

`hookMode`: `forbiddenList` (default; skip `forbiddenPaths` and descendants) / `whitelist` (only `includePaths` descendants). Env `WIKI_HOOK_MODE` / `WIKI_FORBIDDEN_PATHS` / `WIKI_INCLUDE_PATHS` override config.

**ZCode** `~/.zcode/cli/config.json`:

```json
{"hooks":{"enabled":true,"events":{"Stop":[{"hooks":[{"type":"process","command":"/path/to/wiki","args":["check"],"timeoutMs":8000}]}]}}}
```

**Claude Code** `~/.claude/settings.json`:

```json
{"hooks":{"SessionEnd":[{"hooks":[{"type":"command","command":"/path/to/wiki check"}]}]}}
```

Tools without hooks: `wiki inject` (marker-anchored, idempotent). Target: `--file` > `injectFile` / `WIKI_INJECT_FILE` > `~/.qwen/QWEN.md`.

Wiki root: `WIKI_ROOT` > exe dir (has `index.md`) > cwd > exe fallback. Inline hook: `cmd /c "set WIKI_ROOT=D:\vault&& wiki.exe check"`.

## Blog (only when the user explicitly wants to publish)

First `wiki config set blogRepo <absolute path>`.

**Hard rules:**

1. Create with `wiki blog new`, publish with `wiki blog publish`. **Never** Write/Edit a new post under the blog repo `content/post/`.
2. Updates: body and `lastmod` only; do not change front-matter field structure; then `wiki blog publish <file_name>`.
3. Draft the body outside the blog repo, then `--file`.
4. After `wiki blog list`, **reuse** existing categories/tags; do not invent synonyms.
5. Do not retry a failed push: tell the user “committed locally; `git push` in the blog repo”.

```
wiki blog list
wiki blog new --title "..." --slug english-kebab --categories "..." --tags "..." --name file_name --file body.md [--dry-run]
wiki blog publish file_name
```

`--name` / publish args have no `.md`. Slug/filename clashes fail at new (before hugo new). Prose follows the `tech-blog` skill.

## Config cheat sheet

Priority: env > `config.json` > defaults.

| Key | Env | Default |
|---|---|---|
| `blogRepo` | `WIKI_BLOG_REPO` | none |
| `blogPosts` | `WIKI_BLOG_POSTS` | `<blogRepo>/content/post` |
| `hugoBin` | `WIKI_HUGO_BIN` | `hugo` |
| `hugoSite` | `WIKI_HUGO_SITE` | `<blogRepo>` |
| `knowledgeDirs` | `WIKI_KNOWLEDGE_DIRS` | `wiki, issues` |
| `hookMode` | `WIKI_HOOK_MODE` | `forbiddenList` |
| `forbiddenPaths` | `WIKI_FORBIDDEN_PATHS` | empty |
| `includePaths` | `WIKI_INCLUDE_PATHS` | empty |
| `projectGitignore` | `WIKI_PROJECT_GITIGNORE` | `true` |
| `defaultMode` | `WIKI_DEFAULT_MODE` | `link` |
| `injectFile` | `WIKI_INJECT_FILE` | `~/.qwen/QWEN.md` |
| — | `WIKI_ROOT` | see resolution |

Optional opt-in: `<!-- wiki-sync {...} -->` in a project `AGENTS.md` is still honored by `check`.

## Gotchas

- Same-batch calls are independent; dependent steps must be sequential calls.
- Flags may follow positionals (lenient parse).
- Copy mode: files deleted on the project side remain in the wiki.
- Upstream clones that ship `wiki/` / `issues/`: `init` / `sync --fix` migrate then replace with windows — confirm those are not upstream official docs first.
