package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/snap-gs/snap-gs/public/lobby"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	LobbyHelpUse   = "lobby"
	LobbyHelpShort = "lobby short"
	LobbyHelpLong  = `lobby long`
)

var LobbyDefaultExe string

func init() {
	var heads, tails []string
	switch runtime.GOOS {
	case "linux":
		heads = []string{os.Getenv("HOME")}
		tails = []string{
			"/snapshot_server_*.x86_64",
			"/steamapps/common/Snapshot VR Dedicated Server/snapshot_server_*.x86_64",
			"/Steam/steamapps/common/Snapshot VR Dedicated Server/snapshot_server_*.x86_64",
			"/.steam/steamapps/common/Snapshot VR Dedicated Server/snapshot_server_*.x86_64",
			"/.steam/root/steamapps/common/Snapshot VR Dedicated Server/snapshot_server_*.x86_64",
			"/.steam/steam/steamapps/common/Snapshot VR Dedicated Server/snapshot_server_*.x86_64",
			"/.steam/debian-installation/steamapps/common/Snapshot VR Dedicated Server/snapshot_server_*.x86_64",
		}
	case "windows":
		heads = []string{os.Getenv("SYSTEMDRIVE")}
		tails = []string{
			`\SnapshotVR.exe`,
			`\steamapps\common\Snapshot VR\SnapshotVR.exe`,
			`\Program Files*\steam\steamapps\common\Snapshot VR\SnapshotVR.exe`,
		}
	default:
		return
	}
	if heads[0] != "" {
		heads = append(heads, ".")
	} else {
		heads[0] = "."
	}
	for i := 0; LobbyDefaultExe == "" && i < len(heads); i++ {
		for j := 0; LobbyDefaultExe == "" && j < len(tails); j++ {
			matches, _ := filepath.Glob(heads[i] + tails[j])
			for k := 0; LobbyDefaultExe == "" && k < len(matches); k++ {
				LobbyDefaultExe = matches[k]
			}
		}
	}
}

func NewLobbyCommand() *cobra.Command {
	c := cobra.Command{
		Args:  cobra.ExactArgs(0),
		Long:  LobbyHelpLong,
		Short: LobbyHelpShort,
		Use:   LobbyHelpUse,
		RunE:  RunE,
	}
	c.Flags().SortFlags = false
	c.Flags().AddFlagSet(NewLobbyFlagSet(c.Name(), pflag.ContinueOnError))
	c.MarkFlagRequired("roomname")
	return &c
}

func NewLobbyFlagSet(name string, handler pflag.ErrorHandling) *pflag.FlagSet {
	f := pflag.NewFlagSet(name, handler)
	f.SortFlags = false
	f.String("roomname", "", "lobby name")
	f.String("password", "", "lobby auth")
	f.String("specdir", "", "read desired lobby status here")
	f.String("statdir", "", "write current lobby status here")
	f.String("matchdir", "", "write compressed match JSON here")
	f.String("logdir", "", "write compressed lobby logs here")
	f.String("exe", LobbyDefaultExe, "path to SnapshotVR executable")
	f.Duration("timeout", time.Hour*15, "timeout lobby when no players join")
	f.Duration("uptimeout", time.Minute*5, "same as <timeout> when <specdir>/up is set")
	f.Duration("admintimeout", time.Minute*15, "timeout lobby when admin delays match start")
	f.Duration("minuptime", time.Second*15, "min time lobby must run")
	f.Int("maxfails", 3, "max times lobby may fail")
	f.Int("maxidles", -1, "max times lobby may idle")
	f.Bool("debug", false, "enable debug output")
	return f
}

func RunE(cmd *cobra.Command, args []string) error {
	var err error
	var opts lobby.Options
	f := cmd.Flags()
	if opts.Roomname, err = f.GetString("roomname"); err != nil {
		return err
	}
	if opts.Password, err = f.GetString("password"); err != nil {
		return err
	}
	if opts.SpecDir, err = f.GetString("specdir"); err != nil {
		return err
	}
	if opts.StatDir, err = f.GetString("statdir"); err != nil {
		return err
	}
	if opts.MatchDir, err = f.GetString("matchdir"); err != nil {
		return err
	}
	if opts.LogDir, err = f.GetString("logdir"); err != nil {
		return err
	}
	if opts.Exe, err = f.GetString("exe"); err != nil {
		return err
	}
	if opts.Debug, err = f.GetBool("debug"); err != nil {
		return err
	}
	if opts.MaxFails, err = f.GetInt("maxfails"); err != nil {
		return err
	}
	if opts.MaxIdles, err = f.GetInt("maxidles"); err != nil {
		return err
	}
	if opts.MinUptime, err = f.GetDuration("minuptime"); err != nil {
		return err
	}
	if opts.Timeout, err = f.GetDuration("timeout"); err != nil {
		return err
	}
	if opts.MinUpUptime, err = f.GetDuration("uptimeout"); err != nil {
		return err
	}
	if opts.AdminTimeout, err = f.GetDuration("admintimeout"); err != nil {
		return err
	}
	if opts.LogDir != "" {
		if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
			return err
		}
	}
	if opts.MatchDir != "" {
		if err := os.MkdirAll(opts.MatchDir, 0o755); err != nil {
			return err
		}
	} else {
		opts.MatchDir = opts.LogDir
	}
	if opts.StatDir != "" {
		if err := os.MkdirAll(opts.StatDir, 0o755); err != nil {
			return err
		}
		dirents, err := os.ReadDir(opts.StatDir)
		if err != nil {
			return err
		}
		for i := range dirents {
			name := dirents[i].Name()
			if strings.HasPrefix(name, "last") {
				// Preserve last{log,...}
				continue
			}
			file := filepath.Join(opts.StatDir, name)
			last := filepath.Join(opts.StatDir, "last"+name)
			if os.Rename(file, last) == nil {
				continue
			}
			if err := os.Remove(file); err != nil {
				return err
			}
		}
	}
	return lobby.Run(cmd.Context(), cmd.OutOrStdout(), cmd.OutOrStderr(), &opts)
}
