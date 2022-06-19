package cmd

import (
	"os"
	"path/filepath"
	"runtime"
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
	c.MarkFlagRequired("session")
	return &c
}

func NewLobbyFlagSet(name string, handler pflag.ErrorHandling) *pflag.FlagSet {
	f := pflag.NewFlagSet(name, handler)
	f.SortFlags = false
	f.String("session", "", "set lobby name")
	f.String("password", "", "set lobby auth")
	f.String("specdir", "", "read desired status from <specdir>/*")
	f.String("statdir", "", "write current status to <statdir>/*")
	f.String("logdir", "", "write compressed logs to <logdir>/*-lobby.log.gz")
	f.Bool("logstate", false, "write compressed state.json to <logdir>/*-state.json.gz")
	f.Bool("logmatch", false, "write compressed match.json to <logdir>/*-match.json.gz")
	f.Bool("logclean", false, "write anonymized clean.json to <logdir>/*-clean.json.gz")
	f.Int("maxidles", -1, "max idles allowed in total before graceful restart")
	f.Int("maxfails", 3, "max fails allowed in a row across graceful restarts")
	f.Duration("minuptime", time.Second*15, "min uptime expected before graceful restart")
	f.Duration("minupuptime", time.Minute*5, "<minuptime> when <specdir>/up set")
	f.Duration("admintimeout", time.Minute*15, "timeout when admin delays match")
	f.Duration("timeout", time.Hour*15, "timeout when no players join")
	f.String("listen", "0.0.0.0:27002", "bind local ip:port")
	f.String("listen1", "", "bind public ip:port (default <listen>)")
	f.String("listen2", "", "bind accel ip:port (default <listen1>)")
	f.String("exe", LobbyDefaultExe, "path to executable")
	f.Bool("debug", false, "enable debug output")
	return f
}

func RunE(cmd *cobra.Command, args []string) error {
	var err error
	var opts lobby.Options
	f := cmd.Flags()
	if opts.Session, err = f.GetString("session"); err != nil {
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
	if opts.LogDir, err = f.GetString("logdir"); err != nil {
		return err
	}
	if opts.LogState, err = f.GetBool("logstate"); err != nil {
		return err
	}
	if opts.LogMatch, err = f.GetBool("logmatch"); err != nil {
		return err
	}
	if opts.LogClean, err = f.GetBool("logclean"); err != nil {
		return err
	}
	if opts.Listen, err = f.GetString("listen"); err != nil {
		return err
	}
	if opts.Listen1, err = f.GetString("listen1"); err != nil {
		return err
	}
	if opts.Listen2, err = f.GetString("listen2"); err != nil {
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
	if opts.MinUpUptime, err = f.GetDuration("minupuptime"); err != nil {
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
	if opts.StatDir != "" {
		if err := os.MkdirAll(opts.StatDir, 0o755); err != nil {
			return err
		}
	}
	return lobby.Run(cmd.Context(), cmd.OutOrStdout(), cmd.OutOrStderr(), &opts)
}
