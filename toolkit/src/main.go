package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/spock2300/vmake/pkg/plugin"
	"github.com/spock2300/vmake/pkg/toolchain"
)

func Main(ctx *plugin.Context) {
	def, _ := ctx.LoadToolchainDef()

	registerCrossToolchain(ctx)

	ctx.AddSubCommand(&cobra.Command{
		Use:   "toolchains",
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
		Use:   "doctor",
		Short: "Diagnose this plugin's setup",
		Run: func(cmd *cobra.Command, args []string) {
			runDoctor(ctx, def)
		},
	})
}

func runDoctor(ctx *plugin.Context, def *toolchain.ToolchainDef) {
	fmt.Printf("plugin dir: %s\n", ctx.PluginDir)
	fmt.Printf("repo dir:   %s\n", ctx.RepoDir)
	fmt.Printf("vmake dir:  %s\n", ctx.VMakeDir)

	printMergedFiles(filepath.Join(ctx.PluginDir, "src"))

	if def == nil {
		fmt.Println("toolchain.json: absent")
	} else {
		fmt.Printf("toolchain.json: name=%s version=%s cc=%s\n", def.Name, def.Version, def.Tools.CC)
	}

	fmt.Printf("registered toolchains: %d\n", len(ctx.GetToolchains()))

	fmt.Println("tool lookup:")
	for _, name := range []string{"cc", "make", "cmake", "gcc"} {
		path, err := exec.LookPath(name)
		if err != nil {
			fmt.Printf("  %-8s missing\n", name)
			continue
		}
		fmt.Printf("  %-8s %s\n", name, path)
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
