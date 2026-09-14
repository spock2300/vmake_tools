# vmake Extension Plugin Scaffold

A minimal, working extension repository: two plugins, one declarative toolchain,
and a 153 MiB toolchain archive shipped through Git LFS.

Plugins are **interpreted by yaegi at runtime** — no `go build`, no `.so`, no
`go.mod`. `vmake` interprets the sources on every invocation.

## Layout

```
examples/plugins/                          <- an extension REPOSITORY (a git repo)
├── hello/
│   ├── plugin.json                        <- name: hello
│   └── src/main.go                        <- entry: src/main.go
├── tools/
│   ├── plugin.json                        <- name: tools
│   └── src/main.go                        <- entry; Main lives here
├── arm-none-eabi/
│   └── toolchain.json                     <- declarative toolchain + install block
└── assets/toolchains/
    └── arm-none-eabi-15.3.rel1.tar.xz     <- Git LFS object (153 MiB)
```

`vmake` walks `~/.vmake/extensions/<repo>/<subdir>/`. A subdirectory containing
`plugin.json` is a plugin; one containing `toolchain.json` is a toolchain
definition. A repository may hold any mix of both.

## Try it

This directory *is* a git repository, so it can be added directly:

```bash
vmake ext add scaffold file://$PWD/examples/plugins   # or the hosted git URL

vmake hello greet alice
vmake hello where
vmake tools list
vmake tools show
vmake tools doctor
```

Then cross-compile with the shipped toolchain — the 153 MiB archive is fetched
through Git LFS on first use:

```bash
vmake build --toolchain arm-none-eabi
```

Verified end to end: both plugins load, `tools` registers `arm-none-eabi` from
`arm-none-eabi/toolchain.json`, and selecting it while uninstalled triggers
`git lfs pull` → extract → register, after which `arm-none-eabi-gcc` resolves to
`~/.vmake/toolchains/arm-none-eabi-15.3.rel1/bin/arm-none-eabi-gcc`.

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
   `.go` file in `filepath.Dir(entry)` is concatenated into one unit, so one file
   can call helpers declared in another. Import lists are deduplicated; the same
   path imported under two different aliases is an error.

4. **`Main` runs at startup, before cobra parses anything.** Registering
   toolchains, setting missing-toolchain handlers and adding global flags all
   take effect before any command executes. A `Run` closure registered via
   `AddSubCommand` fires later, when the user invokes that subcommand.

5. **A failing plugin is reported but never fatal.** `loadPlugins()` logs to
   stderr and continues:
   ```
   extension plugin 'tools' load failed: yaegi eval plugin tools: ...
   ```
   `vmake` still runs (exit 0 for unrelated commands); only the plugin's own
   command is missing, so the user sees `unknown command "tools"`. A broken
   plugin does **not** affect sibling plugins or other repositories.

6. **Only three import paths are available**: `pkg/plugin`, `pkg/toolchain`,
   `github.com/spf13/cobra` (plus `pflag`). `pkg/api` — the buildscript API — is
   **not** registered for plugins. The yaegi stdlib and `unrestricted` symbol
   sets are available, so `fmt`, `os`, `os/exec`, `path/filepath`, `encoding/json`,
   `net/http` etc. all work.

## What the examples demonstrate

`hello/src/main.go` — the minimum viable plugin: two subcommands, positional
args via `cobra.MaximumNArgs`, reading `ctx.PluginDir` / `ctx.CommandName`.

`tools/src/main.go` — the declarative path. `Main` calls
`ctx.RegisterToolchainsFromRepo()`, which walks the repository root, registers
every `toolchain.json` it finds, and wires auto-download for any that declare an
`install` block. **Without that call a `toolchain.json` is inert** — nothing scans
it implicitly. `tools show` prints the declared definitions;
`tools doctor` also prints registered state and host tool lookup.

Because `arm-none-eabi/toolchain.json` carries the `install` block, selecting
`--toolchain arm-none-eabi` runs the built-in downloader: `git lfs pull` →
`ExtractToDir` → `RegisterToolchain`. No plugin code is involved in the download.

## Shipping a toolchain through Git LFS

`.gitattributes` tracks `assets/toolchains/*.tar.*` with the `lfs` filter, so the
archive becomes a ~134-byte pointer in git history while the bytes live in LFS
storage. Confirm with:

```bash
git lfs ls-files
git cat-file -s HEAD:assets/toolchains/arm-none-eabi-15.3.rel1.tar.xz   # -> 134
```

Four constraints govern the archive, and all of them come from
`makeAutoDownload` in `cmd/vmake/ext_cmd.go` plus `ToolchainDef.InstallDir`:

1. **It must live at the repository root**, in `assets/toolchains/`. The download
   path is hard-coded as `<repoDir>/assets/toolchains/<install.file>` — assets
   inside `tools/` would not be found.
2. **`install.file` is resolved relative to that directory**, so it is a bare
   filename, never a path.
3. **Extraction does no top-level stripping.** The toolchain is installed at
   `~/.vmake/toolchains/<name>-<version>/`, and `RegisterToolchain` only sets
   `InstallPath` when that exact directory exists (`ToolchainDef.ToToolchain`).
   The archive's single top-level directory must therefore be named
   `arm-none-eabi-15.3.rel1`, matching `name` + `version` in `toolchain.json`.
   The upstream archive instead extracts
   `arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi/`, so it was repacked with
   the top-level directory renamed — file modes preserved, contents otherwise
   byte-identical. Getting this wrong fails quietly: the downloader prints
   `Toolchain ... installed to ...` while `InstallPath` stays empty and the build
   dies later with `exec: "arm-none-eabi-gcc": executable file not found in $PATH`.
4. **`Tools.*` names are looked up as `<InstallPath>/bin/<tool>`** and fall back
   to `PATH` (`ResolveToolPath`). A `prefix` alone is not enough for optional
   tools: `objcopy`/`size`/`nm` etc. are resolved from `tc.Prefix + <TOOLNAME>`,
   i.e. `arm-none-eabi-objcopy`. Declare them explicitly in `tools` for clarity.

Note: `install.sha256` exists in the schema and is parsed into
`InstallConfig.Sha256`, but **nothing in vmake ever verifies it**. It is omitted
here rather than implying an integrity check that does not happen. The SHA256 of
this archive is
`563bebb2b97d53382b956d6ee1fe61e2cae26699901417234a37df505ef9b5fa`.

## Pitfalls

- **Global flags are global.** `ctx.AddGlobalCFlags` / `AddGlobalCxxFlags` /
  `AddGlobalLdFlags` affect *every* target in *every* project on the machine, not
  just the plugin's own builds. Register them deliberately. (The official `tc`
  extension uses this to inject `-include vmake_std.h` into all C and C++
  compilations.)
- **`vmakeDir` is `$HOME/.vmake`**, so `HOME` is the way to sandbox a test.
- **`vmake ext add` requires a git URL** — it runs `git clone`. A plain directory
  path will not work; use `file:///abs/path`.
- **`SetOnMissing` is dead code for an installed toolchain.** `Manager.SelectToolchain`
  consults the `onMissing` handler only when `InstallPath` is empty. The declarative
  `install` block is the better path precisely because it avoids this trap.
  `SetOnMissing` is keyed by toolchain name, so several plugins can each own the
  install strategy for their own toolchain.
- **Editing a plugin requires no rebuild**, but the extension repo is a clone:
  after committing upstream, run `vmake ext update` to pull, or edit
  `~/.vmake/extensions/<repo>/` directly while iterating.
- **A half-finished download leaves a stray directory.** Extraction happens before
  `InstallPath` is checked, so a failed run can leave
  `~/.vmake/toolchains/<archive-top-level>/` behind. Remove it before retrying.
