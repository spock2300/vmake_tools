package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spock2300/vmake/pkg/plugin"
	"github.com/spock2300/vmake/pkg/toolchain"
)

func registerCrossToolchain(ctx *plugin.Context) {
	def := &toolchain.ToolchainDef{
		Name:        "riscv-none-elf",
		Version:     "13.2.0",
		DisplayName: "RISC-V bare-metal GCC 13.2.0",
		Host:        "x86_64-linux-gnu",
		Prefix:      "riscv-none-elf",
		Tools: toolchain.Tools{
			CC:  "riscv-none-elf-gcc",
			CXX: "riscv-none-elf-g++",
			AR:  "riscv-none-elf-ar",
			NM:  "riscv-none-elf-nm",
		},
		DefaultFlags: toolchain.DefaultFlags{
			CFlags:   []string{"-march=rv32imac", "-mabi=ilp32", "-Os"},
			CxxFlags: []string{"-march=rv32imac", "-mabi=ilp32", "-Os"},
			LdFlags:  []string{"-march=rv32imac", "-mabi=ilp32"},
		},
	}

	toolchainsDir := filepath.Join(ctx.VMakeDir, "toolchains")
	ctx.RegisterToolchain(def.Name, def.ToToolchain(toolchainsDir))

	ctx.SetOnMissing(def.Name, func(name string) (*toolchain.Toolchain, error) {
		archive := filepath.Join(ctx.RepoDir, "assets", "toolchains", name+".tar.gz")
		if _, err := os.Stat(archive); err != nil {
			return nil, fmt.Errorf("toolchain %q is not installed and %s is absent; install riscv-none-elf-gcc and put it on PATH", name, archive)
		}

		fmt.Printf("installing toolchain %s from %s\n", name, archive)
		if err := ctx.RunGitLFS(ctx.RepoDir, "pull", "--include", "assets/toolchains/"+name+".tar.gz"); err != nil {
			return nil, fmt.Errorf("git lfs pull for %s: %w", name, err)
		}
		if err := ctx.ExtractToDir(archive, toolchainsDir, "tar.gz"); err != nil {
			return nil, fmt.Errorf("extract %s: %w", archive, err)
		}

		tc := def.ToToolchain(toolchainsDir)
		ctx.RegisterToolchain(def.Name, tc)
		return tc, nil
	})
}
