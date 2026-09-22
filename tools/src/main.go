package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/spock2300/vmake/pkg/plugin"
	"github.com/spock2300/vmake/pkg/toolchain"
)

func Main(ctx *plugin.Context) {
	ctx.RegisterToolchainsFromRepo()

	ctx.AddSubCommand(&cobra.Command{
		Use:   "list",
		Short: "List toolchains visible to vmake",
		Run: func(cmd *cobra.Command, args []string) {
			tcs := ctx.GetToolchains()
			if len(tcs) == 0 {
				fmt.Println("no toolchains registered")
				return
			}
			for name, tc := range tcs {
				fmt.Printf("  %-24s %-28s prefix=%q\n", name, tc.DisplayName, tc.Prefix)
			}
		},
	})

	ctx.AddSubCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the declarative toolchains in this repository",
		Run: func(cmd *cobra.Command, args []string) {
			printDeclared(ctx)
		},
	})

	ctx.AddSubCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Diagnose this plugin's setup",
		Run: func(cmd *cobra.Command, args []string) {
			printMergedFiles(filepath.Join(ctx.PluginDir, "src"))
			printDeclared(ctx)
			printRegistered(ctx)
			printHostTools()
		},
	})
}

func printDeclared(ctx *plugin.Context) {
	defs, err := toolchain.ScanRepoToolchains(ctx.RepoDir)
	if err != nil {
		fmt.Printf("toolchains: %v\n", err)
		return
	}
	fmt.Printf("installation host: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("declared toolchains (%d):\n", len(defs))
	for i := range defs {
		def := &defs[i]
		install := "manual"
		selected, err := def.Installation()
		if err != nil {
			install = err.Error()
		} else if selected != nil {
			install = fmt.Sprintf("%s file=%s root_dir=%s", selected.Method, selected.File, selected.RootDir)
		}
		fmt.Printf("  %-16s version=%-10s install=%s\n", def.Name, def.Version, install)
	}
}

func printRegistered(ctx *plugin.Context) {
	tcs := ctx.GetToolchains()
	fmt.Printf("registered toolchains (%d):\n", len(tcs))
	for name, tc := range tcs {
		state := "not installed"
		if tc.InstallPath != "" {
			state = tc.InstallPath
		}
		fmt.Printf("  %-16s %s\n", name, state)
	}
}

func printHostTools() {
	fmt.Println("host tools:")
	for _, name := range []string{"cc", "make", "cmake", "arm-none-eabi-gcc"} {
		path, err := exec.LookPath(name)
		if err != nil {
			fmt.Printf("  %-20s missing\n", name)
			continue
		}
		fmt.Printf("  %-20s %s\n", name, path)
	}
}

func printMergedFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("merged go files: %v\n", err)
		return
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		names = append(names, e.Name())
	}
	fmt.Printf("merged go files (%d): %v\n", len(names), names)
}
