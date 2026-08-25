// Package commands wires up the ruust cobra command tree.
//
// This file owns the root command and the shared client/config plumbing. The
// subcommands (login, eggs, deploy, logs, regions, and so on) live in sibling
// files in this package and register themselves by calling AddCommand in their
// own init() functions, for example:
//
//	func init() { AddCommand(loginCmd) }
//
// With no subcommand, ruust launches the interactive TUI when a session token
// is present, otherwise it prints a friendly 'ruust login' hint.
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/config"
	"github.com/RuustRun/cli/internal/tui"
	"github.com/RuustRun/cli/internal/ui"
)

// hostFlag holds the value of the global --host flag.
var hostFlag string

// cfg is the config loaded once in PersistentPreRun and shared by all commands.
var cfg *config.Config

// rootCmd is the base 'ruust' command.
var rootCmd = &cobra.Command{
	Use:   "ruust",
	Short: "Ruust: deploy an app as an Egg and watch it hatch",
	Long: "ruust is the command line for Ruust, where you deploy a git repo as an Egg\n" +
		"and it goes from incubating to hatched with unmetered egress on a flat monthly price.",
	SilenceUsage:  true,
	SilenceErrors: true,
	// PersistentPreRunE loads config for every command and applies the --host
	// flag override before the command runs.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if hostFlag != "" {
			c.Host = hostFlag
		}
		cfg = c
		return nil
	},
	// RunE with no subcommand launches the TUI if signed in, otherwise hints.
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Token(cfg) == "" {
			printLoginHint()
			return nil
		}
		return tui.Run(Client())
	},
}

// Execute runs the root command. It is the single entry point called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.StateStyle("cracked").Render("error:")+" "+err.Error())
		os.Exit(1)
	}
}

// AddCommand registers a subcommand on the root. Sibling files in this package
// call this from their init() to contribute their commands.
func AddCommand(cmds ...*cobra.Command) {
	rootCmd.AddCommand(cmds...)
}

// RootCmd exposes the root command for subcommand files that need to attach
// flags or inspect it directly.
func RootCmd() *cobra.Command { return rootCmd }

// Config returns the config loaded in PersistentPreRun. It is nil until a
// command runs, so only call it from within a command's Run.
func Config() *config.Config { return cfg }

// Client builds an API client from the loaded config, applying environment
// overrides. Call it from within a command's Run.
func Client() *api.Client { return api.NewFromConfig(cfg) }

// printLoginHint prints the ember logo and a friendly nudge to sign in.
func printLoginHint() {
	fmt.Println(ui.Logo())
	fmt.Println()
	fmt.Println(ui.Bone.Render("You are not signed in yet."))
	fmt.Println(ui.Subtle.Render("Run ") + ui.Key.Render("ruust login") +
		ui.Subtle.Render(" to sign in, then run ") + ui.Key.Render("ruust") +
		ui.Subtle.Render(" to open your Eggs."))
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hostFlag, "host", "",
		"Ruust API host (overrides config; env RUUST_HOST wins over this)")
}
