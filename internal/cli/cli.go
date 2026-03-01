// Package cli provides CLI utilities for AgentAPI using phenotype-go-kit.
package cli

import (
	"github.com/spf13/cobra"
)

// CreateRootCommand creates the root command for AgentAPI.
// Note: Directly creating the command to avoid compatibility issues with different Cobra versions.
//
// Parameters:
//   - version: The version of AgentAPI
//
// Returns:
//   - *cobra.Command: The root command
func CreateRootCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:     "agentapi",
		Short:   "AgentAPI CLI",
		Long:    `AgentAPI - HTTP API for Claude Code, Goose, Aider, Gemini and Codex`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default behavior: show help if no subcommand is provided
			return cmd.Help()
		},
	}
}

// CommandBuilder provides a fluent interface for building AgentAPI commands.
// This is a simplified version that avoids compatibility issues with different Cobra versions.
type CommandBuilder struct {
	cmd *cobra.Command
}

// NewCommandBuilder creates a new CommandBuilder.
//
// Parameters:
//   - use: The command name
//
// Returns:
//   - *CommandBuilder: A new CommandBuilder instance
func NewCommandBuilder(use string) *CommandBuilder {
	return &CommandBuilder{
		cmd: &cobra.Command{
			Use: use,
		},
	}
}

// Short sets the short description of the command.
func (cb *CommandBuilder) Short(short string) *CommandBuilder {
	cb.cmd.Short = short
	return cb
}

// Long sets the long description of the command.
func (cb *CommandBuilder) Long(long string) *CommandBuilder {
	cb.cmd.Long = long
	return cb
}

// RunE sets the RunE function of the command.
func (cb *CommandBuilder) RunE(runFunc func(cmd *cobra.Command, args []string) error) *CommandBuilder {
	cb.cmd.RunE = runFunc
	return cb
}

// Build returns the constructed cobra command.
func (cb *CommandBuilder) Build() *cobra.Command {
	return cb.cmd
}

// AddStringFlag adds a string flag to the command.
func (cb *CommandBuilder) AddStringFlag(name string, shorthand string, defaultValue string, usage string) *CommandBuilder {
	cb.cmd.Flags().StringP(name, shorthand, defaultValue, usage)
	return cb
}

// AddBoolFlag adds a boolean flag to the command.
func (cb *CommandBuilder) AddBoolFlag(name string, shorthand string, defaultValue bool, usage string) *CommandBuilder {
	cb.cmd.Flags().BoolP(name, shorthand, defaultValue, usage)
	return cb
}

// AddIntFlag adds an integer flag to the command.
func (cb *CommandBuilder) AddIntFlag(name string, shorthand string, defaultValue int, usage string) *CommandBuilder {
	cb.cmd.Flags().IntP(name, shorthand, defaultValue, usage)
	return cb
}

// AddSubcommand adds a subcommand to the command.
func (cb *CommandBuilder) AddSubcommand(subCmd *cobra.Command) *CommandBuilder {
	cb.cmd.AddCommand(subCmd)
	return cb
}
