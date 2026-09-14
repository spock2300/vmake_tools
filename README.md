# vmake Extension Plugins

A working vmake extension repository: two plugins, one declarative toolchain, and
the upstream ARM GNU Toolchain archive (153 MiB) shipped through Git LFS.

Plugins are **interpreted by yaegi at runtime** — no `go build`, no `.so`, no
`go.mod`. `vmake` interprets the sources on every invocation.

## Layout

```
.
├── hello/
│   ├── plugin.json                        <- name: hello
│   └── src/main.go                        <- entry: src/main.go
├── tools/
│   ├── plugin.json                        <- name: tools
│   └── src/main.go                        <- entry; Main lives here
├── arm-none-eabi/
│   └── toolchain.json                     <- declarative toolchain + install block
└── assets/toolchains/
    └── arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi.tar.xz   <- Git LFS object
```

`vmake` walks `~/.vmake/extensions/<repo>/<subdir>/`. A subdirectory containing
`plugin.json` is a plugin; one containing `toolchain.json` is a toolchain
definition. A repository may hold any mix of both.

## Usage

```bash
vmake ext add vmake-tools git@github.com:spock2300/vmake_tools.git

vmake hello greet alice
vmake hello where
vmake tools list
vmake tools show
vmake tools doctor
```

Cross-compile with the shipped toolchain — the archive is fetched through Git LFS
on first use:

```bash
vmake build --toolchain arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi
```

Verified end to end: the toolchain auto-downloads, extracts, registers, and
compiles and links `main.c` into `ELF 32-bit LSB executable, ARM, EABI5`.

## `plugin.json`

| Field | Required | Notes |
|-------|----------|-------|
| `name` | yes | Also the root command name: `vmake <name>` |
| `entry` | yes | Path to the entry file, e.g. `src/main.go` |
| `enabled` | **yes in practice** | Defaults to `false`; **a plugin with `enabled` omitted or `false` is skipped silently** |
| `version` | no | Shown by `vmake ext list` |
| `description` | no | Becomes the root command's `Short` text |

## The plugin contract

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

`hello/src/main.go` — the minimum viable plugin: two subcommands, positional args
via `cobra.MaximumNArgs`, reading `ctx.PluginDir` / `ctx.CommandName`.

`tools/src/main.go` — the declarative path. `Main` calls
`ctx.RegisterToolchainsFromRepo()`, which walks the repository root, registers
every `toolchain.json` it finds, and wires auto-download for any that declare an
`install` block. **Without that call a `toolchain.json` is inert** — nothing scans
it implicitly. `tools show` prints the declared definitions; `tools doctor` also
prints registered state plus host tool lookup.

Because `arm-none-eabi/toolchain.json` carries the `install` block, selecting the
toolchain runs vmake's built-in downloader (`makeAutoDownload` in
`cmd/vmake/ext_cmd.go`): `git lfs pull` → `ExtractToDir` → `RegisterToolchain`.
No plugin code participates in the download.

## The archive is shipped pristine — and that dictates the toolchain name

Git LFS stores the **unmodified upstream archive**:
`arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi.tar.xz`, sha256
`563bebb2b97d53382b956d6ee1fe61e2cae26699901417234a37df505ef9b5fa`.
`git lfs ls-files` lists it; the blob in git history is a 134-byte pointer.

Three facts about `makeAutoDownload` and `ToolchainDef` force the rest of the
configuration, and none of them are optional:

1. **Assets must live at the repository root**, in `assets/toolchains/`. The
   download path is hard-coded as `<repoDir>/assets/toolchains/<install.file>`.
   `install.file` is a bare filename, never a path.

2. **Extraction does no top-level stripping.** The toolchain is installed at
   `~/.vmake/toolchains/` and only recognised when the directory
   `<name>-<version>` exists there — `ToolchainDef.ToToolchain` sets `InstallPath`
   only if that exact path is a real directory. `ToToolchain` appends `-<version>`
   when `version` is non-empty, so the directory name vmake looks for is
   either `<name>` or `<name>-<version>`.

3. **No plugin code can rename that directory** on the declarative path, because
   the downloader is pure vmake (`RegisterToolchainsFromRepo` → `SetOnMissing` →
   `makeAutoDownload`).

The upstream archive extracts exactly one top-level directory,
`arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi/`. Since the archive is kept
pristine, the only way to satisfy facts 2 and 3 together is for `name` to be
literally that directory name, with `version` omitted so no suffix is appended:

```json
"name": "arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi"
```

Hence the long `--toolchain` value — it is not cosmetic. Renaming it to something
shorter (`arm-none-eabi`) or adding a `version` breaks the install path, and the
failure is **silent and confusing**: the downloader prints
`Toolchain ... installed to ...` while `InstallPath` stays empty, and the build
dies later with `exec: "arm-none-eabi-gcc": executable file not found in $PATH`
because `ResolveToolPath` fell through to `PATH`.

The other two fields are free choices:
`display_name` carries the readable label, and `prefix: "arm-none-eabi"` supplies
the compiler prefix for optional tools — `resolveOptionalTool` falls back to
`tc.Prefix + <TOOLNAME>` (`arm-none-eabi-objcopy`), and the `tools` block declares
them explicitly anyway.

`ldflags` includes `--specs=nosys.specs`: without it, linking a bare-metal
`main.c` fails with `undefined reference to '_exit'` from newlib's `libc_a-exit.o`.

Note: `install.sha256` exists in the schema and is parsed into
`InstallConfig.Sha256`, but **nothing in vmake ever verifies it** — it is omitted
rather than implying an integrity check that does not happen.

## Pitfalls

- **Global flags are global.** `ctx.AddGlobalCFlags` / `AddGlobalCxxFlags` /
  `AddGlobalLdFlags` affect *every* target in *every* project on the machine, not
  just the plugin's own builds. Register them deliberately. (The official `tc`
  extension uses this to inject `-include vmake_std.h` into all C and C++
  compilations.)
- **`vmakeDir` is `$HOME/.vmake`**, so `HOME` is the way to sandbox a test.
- **`vmake ext add` requires a git URL** — it runs `git clone`. A plain directory
  path will not work; use `file:///abs/path`.
- **`SetOnMissing` is dead code for an installed toolchain.**
  `Manager.SelectToolchain` consults the `onMissing` handler only when
  `InstallPath` is empty, which is exactly why the declarative `install` block is
  the right mechanism here.
- **Editing a plugin requires no rebuild**, but the extension repo is a clone:
  after committing upstream, run `vmake ext update` to pull, or edit
  `~/.vmake/extensions/<repo>/` directly while iterating.
- **A half-finished download leaves a stray directory.** Extraction happens before
  `InstallPath` is checked, so a failed run can leave
  `~/.vmake/toolchains/<archive-top-level>/` behind. Remove it before retrying.
