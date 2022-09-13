package cmd

import (
	"github.com/snap-gs/snap-gs/internal/lobby"
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

// ExitCode converts errors to exit codes per https://www.freedesktop.org/software/systemd/man/systemd.exec.html
func ExitCode(err error) int {
	switch err {
	case nil, lobby.ErrLobbyDone:
		return 0
	case lobby.ErrLobbyDowned:
		return 4 // EXIT_NOPERMISSION
	case lobby.ErrLobbyStopped:
		return 5 // EXIT_NOTINSTALLED
	case lobby.ErrLobbyRestarted:
		return 6 // EXIT_NOTCONFIGURED
	case lobby.ErrLobbyDisconnected:
		return 7 // EXIT_NOTRUNNING
	case lobby.ErrLobbyBad:
		return 70 // EX_SOFTWARE
	case lobby.ErrLobbyMaxFails:
		return 100
	case lobby.ErrLobbyTimeout:
		return 101
	case lobby.ErrLobbyIdleTimeout:
		return 102
	case lobby.ErrLobbyAdminTimeout:
		return 103
	default:
		return 1
	}
}
