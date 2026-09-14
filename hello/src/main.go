package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/spock2300/vmake/pkg/plugin"
)

func Main(ctx *plugin.Context) {
	ctx.AddSubCommand(&cobra.Command{
		Use:   "greet [name]",
		Short: "Print a greeting",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := "world"
			if len(args) == 1 {
				name = args[0]
			}
			fmt.Printf("hello, %s\n", name)
			fmt.Printf("  plugin dir: %s\n", ctx.PluginDir)
			fmt.Printf("  command:    %s\n", ctx.CommandName)
		},
	})

	ctx.AddSubCommand(&cobra.Command{
		Use:   "where",
		Short: "Show the directories this plugin sees",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("VMakeDir:  %s\n", ctx.VMakeDir)
			fmt.Printf("RepoDir:   %s\n", ctx.RepoDir)
			fmt.Printf("PluginDir: %s\n", ctx.PluginDir)
		},
	})
}
