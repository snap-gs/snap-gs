package cmd

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	c := NewRootCommand()
	c.AddCommand(NewLobbyCommand())
	return c
}

func NewRootCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "snap-gs",
		Short: "root short",
		Long:  `root long`,
	}
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.CompletionOptions.DisableDefaultCmd = true
	c.SetHelpCommand(&cobra.Command{Hidden: true})
	c.PersistentFlags().Bool("debug", false, "enable debug output")
	return c
}
