# vmake Extension Plugins

A working vmake extension repository: two plugins, one declarative toolchain, and
the upstream ARM GNU Toolchain archives for Linux and Windows shipped through Git LFS.

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
│   └── toolchain.json                     <- declarative toolchain + host installations
└── assets/toolchains/
    ├── arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi.tar.xz
    └── arm-gnu-toolchain-15.3.rel1-mingw-w64-x86_64-arm-none-eabi.zip
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
vmake build --toolchain arm-none-eabi
```

The compiler runs on the current host and produces bare-metal ARM ELF output.
The target platform belongs to the project, not the toolchain: define global
options `target_os` (`"none"` for bare metal) and `target_triple`
(`"arm-none-eabi"`) in `build.go`, together with `-mcpu`/`-mthumb` flags. The
same compiler definition serves any Cortex-M project.

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

`tools/src/main.go` — read-only diagnostics. `tools list` prints the registered
toolchains from `ctx.GetToolchains()`; `tools show` / `tools doctor` scan this
repository for `toolchain.json` definitions and report the selected host archive
plus installation state. It registers nothing: `vmake` itself scans every
extension repository and registers the definitions before plugins load. The core
owns downloading, extraction, validation, and registration.

## Toolchain definition

The stable name is `arm-none-eabi`, version `15.3.rel1`, and `prefix` is
`arm-none-eabi-`, including the trailing hyphen expected by `CROSS_COMPILE`.
`toolchain.json` describes only **which programs to run**:

```json
{
  "name": "arm-none-eabi",
  "version": "15.3.rel1",
  "prefix": "arm-none-eabi-",
  "tools": { "cc": "arm-none-eabi-gcc", "ld": "arm-none-eabi-gcc", ... },
  "installations": { "linux/amd64": { ... }, "windows/amd64": { ... } }
}
```

`target_os`, `target_triple` and `default_flags` are **rejected** in this file —
they describe a project, not a compiler. Put them in `build.go`:

```go
func Main(p *api.Package) {
    p.OnConfig(func(ctx *api.ConfigContext) {
        ctx.GlobalOption(api.TargetOSOptionName).SetType(api.OptionString).SetDefault("none")
        ctx.GlobalOption(api.TargetTripleOptionName).SetType(api.OptionString).SetDefault("arm-none-eabi")
        ctx.AddGlobalCFlags("-mcpu=cortex-m4", "-mthumb")
        ctx.AddGlobalCxxFlags("-mcpu=cortex-m4", "-mthumb")
        ctx.AddGlobalLdFlags("-mcpu=cortex-m4", "-mthumb", "--specs=nosys.specs")
    })
}
```

`--specs=nosys.specs` lets basic bare-metal programs link against newlib's syscall
stubs. It does not make the output a host executable.

`installations` selects archives by the **vmake process host**, independently of
the compilation target:

| Host | Archive | `root_dir` |
|------|---------|------------|
| `linux/amd64` | `arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi.tar.xz` | `arm-gnu-toolchain-15.3.rel1-x86_64-arm-none-eabi` |
| `windows/amd64` | `arm-gnu-toolchain-15.3.rel1-mingw-w64-x86_64-arm-none-eabi.zip` | `.` |

The Windows archive stores `bin/` directly at its root. Both upstream archives
remain unmodified and live in `assets/toolchains/`, managed by Git LFS. The
[official release downloads](https://gitlab.arm.com/tooling/gnu-toolchains-for-arm/-/tree/releases/15.3.rel1)
provide the matching archives and checksums. Each
installation declares its SHA256, which vmake verifies before extraction.

Installation uses a temporary directory, checks the declared root and executable
tools, then atomically publishes to:

```
~/.vmake/toolchains/<host-os>/<host-arch>/arm-none-eabi/15.3.rel1/
```

A missing or Git LFS pointer asset triggers a fetch of only the selected archive.
An already materialized local archive is used directly. Failed extraction,
checksum validation, or executable validation leaves no published installation.
An installed toolchain never resolves a missing compiler from another toolchain
on `PATH`.

Unsupported hosts and invalid manifests report errors. Old `host` and `install`
fields must be migrated to `installations`; `target_os`, `target_triple` and
`default_flags` must be moved into the project's `build.go`. Old installations in
a flat toolchains directory are not reused across hosts.

## Pitfalls

- **Global flags are global.** `ctx.AddGlobalCFlags` / `AddGlobalCxxFlags` /
  `AddGlobalLdFlags` affect *every* target in *every* project on the machine, not
  just the plugin's own builds. Register them deliberately; keep CPU/ABI options
  in the project's `build.go` instead.
- **`vmakeDir` is the OS user home plus `.vmake`**; use an isolated home for integration tests.
- **`vmake ext add` requires a git URL** — it runs `git clone`. A plain directory
  path will not work; use `file:///abs/path`.
- **`SetOnMissing` is for hand-rolled installation flows.** Definitions with an
  `installations` entry for the current host get a missing handler automatically;
  use `ctx.SetOnMissing` only when installation cannot be expressed in
  `toolchain.json`.
- **Editing a plugin requires no rebuild**, but the extension repo is a clone:
  after committing upstream, run `vmake ext update` to pull, or edit
  `~/.vmake/extensions/<repo>/` directly while iterating.
- **Git for Windows and Developer Mode remain prerequisites for full builds.**
  The Windows ARM archive supplies its compiler executables; project and package
  storage still requires working symlinks.
