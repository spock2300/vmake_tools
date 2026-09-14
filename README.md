# vmake Extension Plugin Scaffold

A minimal, verified working extension repository. Two plugins and one declarative
toolchain definition, showing the two ways to extend vmake.

Plugins are **interpreted by yaegi at runtime** — no `go build`, no `.so`, no
`go.mod`. `vmake` interprets the sources on every invocation.

## Layout

```
examples/plugins/                     <- an extension REPOSITORY (a git repo)
├── hello/
│   ├── plugin.json                   <- name: hello
│   └── src/main.go                   <- entry: src/main.go
├── toolkit/
│   ├── plugin.json                   <- name: toolkit
│   └── src/
│       ├── main.go                   <- entry; Main lives here
│       └── toolchain.go              <- second file, merged in automatically
└── riscv-none-elf/
    └── toolchain.json                <- declarative toolchain (no plugin code)
```

`vmake` walks `~/.vmake/extensions/<repo>/<subdir>/`. A subdirectory containing
`plugin.json` is a plugin; one containing `toolchain.json` is a toolchain
definition. A repository may hold any mix of both.

## Try it

`vmake ext add` runs `git clone`, so the source directory itself is **not** directly
addable. `examples/plugins/.git-scaffold` preserves this scaffold's own commit
history under a non-standard name — if it stayed `.git`, git would record the whole
directory as a gitlink inside the vmake repository instead of as plain files.
Copy it out and make it a real repo:

```bash
cp -r examples/plugins /tmp/myext && rm -rf /tmp/myext/.git-scaffold
cd /tmp/myext && git init && git add -A && git commit -m "my extensions"

vmake ext add myext file:///tmp/myext
vmake hello greet alice
vmake hello where
vmake toolkit doctor
vmake toolkit toolchains
vmake toolchain list                                  # riscv-none-elf shows up
```

Verified against a real build: both plugins load, subcommands execute, `toolkit`
registers `riscv-none-elf` globally, and `SetOnMissing` fires when that toolchain
is selected while uninstalled.

## `plugin.json`

| Field | Required | Notes |
|-------|----------|-------|
| `name` | yes | Also the root command name: `vmake <name>` |
| `entry` | yes | Path to the entry file, e.g. `src/main.go` |
| `enabled` | **yes in practice** | Defaults to `false`; **a plugin with `enabled` omitted or `false` is skipped silently** |
| `version` | no | Shown by `vmake ext list` |
| `description` | no | Becomes the root command's `Short` text |

## The contract

These are the rules the loader actually enforces (`pkg/plugin/loader.go`).

1. **The import path must be exactly `github.com/spock2300/vmake/pkg/plugin`.**
   Symbols are registered under that literal string and yaegi indexes binary
   packages by exact path, so any other path fails to resolve. The module was
   once `gitee.com/spock2300/vmake`; that spelling **no longer works**:
   ```
   import "gitee.com/spock2300/vmake/pkg/plugin" error: unable to find source related to: ...
   ```
   The same applies to `github.com/spock2300/vmake/pkg/toolchain`.

2. **`Main` must be `func(*plugin.Context)`** and declared at top level. Anything
   else reports `Main has wrong signature: <type>`.

3. **The whole `entry` directory is merged, not just `entry`.** Every non-`_test.go`
   `.go` file in `filepath.Dir(entry)` is concatenated into one unit, so
   `main.go` can call `registerCrossToolchain` from `toolchain.go`. Import lists
   are deduplicated; the same path imported under two different aliases is an
   error.

4. **`Main` runs at startup, before cobra parses anything.** Registering
   toolchains, setting missing-toolchain handlers and adding global flags all
   take effect before any command executes. A `Run` closure registered via
   `AddSubCommand` fires later, when the user invokes that subcommand.

5. **A failing plugin is reported but never fatal.** `loadPlugins()` logs to
   stderr and continues:
   ```
   extension plugin 'toolkit' load failed: yaegi eval plugin toolkit: ...
   ```
   `vmake` still runs (exit 0 for unrelated commands); only the plugin's own
   command is missing, so the user sees `unknown command "toolkit"`. A broken
   plugin does **not** affect sibling plugins or other repositories.

6. **Only three import paths are available**: `pkg/plugin`, `pkg/toolchain`,
   `github.com/spf13/cobra` (plus `pflag`). `pkg/api` — the buildscript API — is
   **not** registered for plugins. The yaegi stdlib and `unrestricted` symbol
   sets are available, so `fmt`, `os`, `os/exec`, `path/filepath`, `encoding/json`,
   `net/http` etc. all work.

## What the examples demonstrate

`hello/src/main.go` — the minimum viable plugin: two subcommands, positional
args via `cobra.MaximumNArgs`, reading `ctx.PluginDir` / `ctx.CommandName`.

`toolkit/src/main.go` — cross-file calls into `toolchain.go`, `exec.LookPath`,
`LoadToolchainDef`, `GetToolchains`, plus a diagnostic that prints the merged
file list.

`toolkit/src/toolchain.go` — `RegisterToolchain` + `SetOnMissing`. Note how the
missing-toolchain handler is reached: `ToolchainDef.ToToolchain` sets
`InstallPath` **only when the install directory already exists**, and
`Manager.SelectToolchain` consults the `onMissing` handler exactly when
`InstallPath` is empty. Registering a handler for an already-installed toolchain
is therefore dead code. `SetOnMissing` is keyed by toolchain name, so several
plugins can each own the install strategy for their own toolchain.

`riscv-none-elf/toolchain.json` — the declarative alternative to
`RegisterToolchain`. Nothing scans it automatically: a plugin must call
`ctx.RegisterToolchainsFromRepo()`, which walks the repo root, registers every
`toolchain.json`, and wires auto-download for any that declare an `install`
block. Without that call the file is inert.

## Pitfalls

- **Global flags are global.** `ctx.AddGlobalCFlags` / `AddGlobalCxxFlags` /
  `AddGlobalLdFlags` affect *every* target in *every* project on the machine, not
  just the plugin's own builds. Register them deliberately. (The official `tc`
  extension uses this to inject `-include vmake_std.h` into all C and C++
  compilations.)
- **`vmakeDir` is `$HOME/.vmake`**, so `HOME` is the way to sandbox a test.
- **`vmake ext add` requires a git URL** — it runs `git clone`. A plain directory
  path will not work; use `file:///abs/path`.
- **Editing a plugin requires no rebuild**, but the extension repo is a clone:
  after committing upstream, run `vmake ext update` to pull, or edit
  `~/.vmake/extensions/<repo>/` directly while iterating.
