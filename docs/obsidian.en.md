> [中文版](obsidian.md)

# Connecting an Obsidian vault

> Two ways to use the wiki root as an Obsidian vault: create the vault first, then point the wiki CLI at it (recommended), or migrate after the fact. Same end state: Obsidian vault root = wiki root, one directory, two jobs.

## Path 1: vault first, then configure the wiki CLI

Create the vault first, then drop wiki CLI persistence into it:

1. Create an empty Obsidian vault. Avoid naming the folder `wiki` — that collides with `knowledgeDirs` type names and auto-discovery may treat the vault itself as a project knowledge dir
2. Point wiki CLI persistence at the vault (user-level env; new terminals pick it up):

```bash
# Windows
setx WIKI_ROOT "D:\path\to\vault"
# macOS / Linux (add to ~/.zshrc / ~/.bashrc)
export WIKI_ROOT=/path/to/vault
```

3. `wiki init <project>` enrolls projects — registry `index.md`, `config.json`, and `projects/` notes all land in the vault and show up in Obsidian immediately
4. Register the session-end hook (`wiki check`) so windows rebuild after a machine switch; run `wiki prepare` by hand when needed
5. Verify: `wiki ls` lists the projects; `obsidian vault=<vault-name> folders` indexes them

## Path 2: order was reversed — migrate existing persistence

Wiki CLI data already exists (for example in the old location). Move it into an Obsidian vault in four steps:

1. **Move data**: `index.md` (registry), `config.json`, `blog.json`, and the whole `projects/` tree into the vault
2. **Set `WIKI_ROOT` explicitly**: the wiki root is resolved via the `index.md` marker (`WIKI_ROOT` env > exe dir > cwd). Moving files without the env var silently breaks the hook:

```bash
# Windows
setx WIKI_ROOT "D:\path\to\vault"
# macOS / Linux
export WIKI_ROOT=/path/to/vault
```

Inline for the hook:

```bash
# Windows (cmd)
cmd /c "set WIKI_ROOT=D:\path\to\vault&& wiki.exe prepare"
cmd /c "set WIKI_ROOT=D:\path\to\vault&& wiki.exe check"
# macOS / Linux
WIKI_ROOT=/path/to/vault wiki prepare
WIKI_ROOT=/path/to/vault wiki check
```

3. **Register the vault**: Open folder as vault in the UI, or edit the registry:

```json
// %APPDATA%\obsidian\obsidian.json (Windows)
{"vaults":{"<16-hex-id>":{"path":"D:\\path\\to\\vault","ts":<millis>,"open":true}},"cli":true}
```

> `obsidian://open?path=` only resolves files in an already-registered vault; it cannot register a new one — don’t spend time on that.

4. **Checklist**:

- `wiki ls`: every project is registered
- `obsidian vault=<vault-name> folders/files`: all notes under `projects/` are indexed
- Final layout:

```
<vault>/
├── .obsidian/
├── index.md                  ← registry (marker)
├── config.json / blog.json
└── projects/                 ← per-project notes (real files, git-friendly)
    ├── project-a/wiki/
    │   └── note.md
    ├── project-b/issues/
    │   └── ...
    └── ...
```

Project side (not inside the Obsidian vault):

```
/path/to/project-a/
├── wiki/                     ← window → <vault>/projects/project-a/wiki
└── .gitignore                ← knowledge-dir ignore entries maintained by wiki
```

> Both paths converge: Obsidian vault root = wiki root, one directory, two jobs. Under live scheme C, Obsidian reads real files in `projects/`. Old scheme A aggregated projects with forward symlinks in the wiki, which is git-hostile; `init` / `sync --fix` invert those into C.
