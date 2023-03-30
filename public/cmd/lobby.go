package cmd

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/public/lobby"
	"github.com/snap-gs/snap-gs/public/options"
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
			"/snapshot*.x86_64",
			"/snapshot*/snapshot*.x86_64",
			"/steamapps/common/Snapshot VR Dedicated Server/snapshot*.x86_64",
			"/steamapps/common/Snapshot VR Dedicated Server/snapshot*/snapshot*.x86_64",
			"/Steam/steamapps/common/Snapshot VR Dedicated Server/snapshot*.x86_64",
			"/Steam/steamapps/common/Snapshot VR Dedicated Server/snapshot*/snapshot*.x86_64",
			"/.steam/steamapps/common/Snapshot VR Dedicated Server/snapshot*.x86_64",
			"/.steam/steamapps/common/Snapshot VR Dedicated Server/snapshot*/snapshot*.x86_64",
			"/.steam/root/steamapps/common/Snapshot VR Dedicated Server/snapshot*.x86_64",
			"/.steam/root/steamapps/common/Snapshot VR Dedicated Server/snapshot*/snapshot*.x86_64",
			"/.steam/steam/steamapps/common/Snapshot VR Dedicated Server/snapshot*.x86_64",
			"/.steam/steam/steamapps/common/Snapshot VR Dedicated Server/snapshot*/snapshot*.x86_64",
			"/.steam/debian-installation/steamapps/common/Snapshot VR Dedicated Server/snapshot*.x86_64",
			"/.steam/debian-installation/steamapps/common/Snapshot VR Dedicated Server/snapshot*/snapshot*.x86_64",
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
	return &c
}

func NewLobbyFlagSet(name string, handler pflag.ErrorHandling) *pflag.FlagSet {
	f := pflag.NewFlagSet(name, handler)
	f.SortFlags = false
	f.String("session", "", "set lobby name")
	f.String("password", "", "set lobby auth")
	f.String("flagdir", "", "read desired --args from <flagdir>")
	f.String("specdir", "", "read desired status from <specdir>")
	f.String("statdir", "", "write current status to <statdir>")
	f.String("logdir", "", "write logs and matches to <logdir>")
	f.String("pidfile", "", "write main[,busy,idle] <pidfile>")
	f.Int("maxfails", 3, "max fails before hard stop")
	f.Duration("minuptime", time.Minute*5, "min uptime before soft stop")
	f.Duration("admintimeout", time.Minute*15, "timeout when admin delays match")
	f.Duration("timeout", time.Hour*15, "timeout when no players join")
	f.String("listen", "", "bind local[,public,accel] ip:port")
	f.String("exe", LobbyDefaultExe, "path to executable")
	f.Bool("debug", false, "enable debug output")
	return f
}

func RunE(cmd *cobra.Command, args []string) error {
	var err error
	var opts options.Lobby
	f := cmd.Flags()
	if opts.Session, err = f.GetString("session"); err != nil {
		return err
	}
	if session := os.Getenv("SNAPGS_LOBBY_SESSION"); session != "" && !f.Changed("session") {
		opts.Session = session
	}
	if opts.Password, err = f.GetString("password"); err != nil {
		return err
	}
	if password := os.Getenv("SNAPGS_LOBBY_PASSWORD"); password != "" && !f.Changed("password") {
		opts.Password = password
	}
	if opts.Listen, err = f.GetString("listen"); err != nil {
		return err
	}
	if listen := os.Getenv("SNAPGS_LOBBY_LISTEN"); listen != "" && !f.Changed("listen") {
		opts.Listen = listen
	}
	if opts.PidFile, err = f.GetString("pidfile"); err != nil {
		return err
	}
	if pidfile := os.Getenv("SNAPGS_LOBBY_PIDFILE"); pidfile != "" && !f.Changed("pidfile") {
		opts.PidFile = pidfile
	}
	if opts.Exe, err = f.GetString("exe"); err != nil {
		return err
	}
	if exe := os.Getenv("SNAPGS_LOBBY_EXE"); exe != "" && !f.Changed("exe") {
		opts.Exe = exe
	}
	if opts.Debug, err = f.GetBool("debug"); err != nil {
		return err
	}
	if debug := os.Getenv("SNAPGS_LOBBY_DEBUG") != ""; debug && !f.Changed("debug") {
		opts.Debug = debug
	}
	if opts.MaxFails, err = f.GetInt("maxfails"); err != nil {
		return err
	}
	if maxfails, err := strconv.Atoi(os.Getenv("SNAPGS_LOBBY_MAXFAILS")); err == nil && !f.Changed("maxfails") {
		opts.MaxFails = maxfails
	}
	if opts.MinUptime, err = f.GetDuration("minuptime"); err != nil {
		return err
	}
	if minuptime, err := time.ParseDuration(os.Getenv("SNAPGS_LOBBY_MINUPTIME")); err == nil && !f.Changed("minuptime") {
		opts.MinUptime = minuptime
	}
	if opts.Timeout, err = f.GetDuration("timeout"); err != nil {
		return err
	}
	if timeout, err := time.ParseDuration(os.Getenv("SNAPGS_LOBBY_TIMEOUT")); err == nil && !f.Changed("timeout") {
		opts.Timeout = timeout
	}
	if opts.AdminTimeout, err = f.GetDuration("admintimeout"); err != nil {
		return err
	}
	if admintimeout, err := time.ParseDuration(os.Getenv("SNAPGS_LOBBY_ADMINTIMEOUT")); err == nil && !f.Changed("admintimeout") {
		opts.AdminTimeout = admintimeout
	}
	if opts.LogDir, err = f.GetString("logdir"); err != nil {
		return err
	}
	if logdir := os.Getenv("SNAPGS_LOBBY_LOGDIR"); logdir != "" && !f.Changed("logdir") {
		opts.LogDir = logdir
	}
	if opts.SpecDir, err = f.GetString("specdir"); err != nil {
		return err
	}
	if specdir := os.Getenv("SNAPGS_LOBBY_SPECDIR"); specdir != "" && !f.Changed("specdir") {
		opts.SpecDir = specdir
	}
	if opts.StatDir, err = f.GetString("statdir"); err != nil {
		return err
	}
	if statdir := os.Getenv("SNAPGS_LOBBY_STATDIR"); statdir != "" && !f.Changed("statdir") {
		opts.StatDir = statdir
	}
	if opts.LogDir != "" {
		if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
			return err
		}
	}
	if opts.SpecDir != "" {
		if err := os.MkdirAll(opts.SpecDir, 0o755); err != nil {
			return err
		}
	}
	if opts.StatDir != "" {
		if err := os.MkdirAll(opts.StatDir, 0o755); err != nil {
			return err
		}
	}
	flagdir, err := f.GetString("flagdir")
	if err != nil {
		return err
	}
	if flagdir2 := os.Getenv("SNAPGS_LOBBY_FLAGDIR"); flagdir2 != "" && !f.Changed("flagdir") {
		flagdir = flagdir2
	}
	if flagdir != "" {
		if err := os.MkdirAll(flagdir, 0o755); err != nil {
			return err
		}
		cancel, err := opts.Watch(cmd.Context(), flagdir)
		if err != nil {
			return err
		}
		defer cancel()
	}
	cgroup := "/sys/fs/cgroup"
	bs, cgroupErr := os.ReadFile("/proc/self/mounts")
	for _, line := range bytes.Split(bs, []byte("\n")) {
		i := bytes.IndexByte(line, ' ')
		if i < 1 {
			continue
		}
		if string(line[:i]) != "cgroup2" {
			continue
		}
		j := bytes.IndexByte(line[i+1:], ' ') + i + 1
		if j < i+2 {
			continue
		}
		cgroup = string(line[i+1 : j])
		break
	}
	if cgroupErr == nil {
		bs, cgroupErr = os.ReadFile("/proc/self/cgroup")
		if cgroupErr == nil {
			cgroup = filepath.Join(cgroup, string(bytes.TrimLeft(bytes.TrimRight(bs, "\n"), "0:/")))
		}
	}
	pidfiles := strings.Split(opts.PidFile, ",")
	for i := range pidfiles {
		p, err := url.Parse(pidfiles[i])
		if err != nil {
			return err
		}
		if p.Scheme == "cgroup" {
			if cgroupErr != nil {
				return cgroupErr
			}
			p.Path = filepath.Join(cgroup, p.Opaque)
			pidfiles[i] = filepath.Join(p.Path, "cgroup.procs")
		}
		if p.Path == "" {
			continue
		}
		if err := os.MkdirAll(p.Path, 0o755); err != nil {
			return err
		}
		if p.Scheme != "cgroup" {
			continue
		}
		if i == 0 {
			if err := os.WriteFile(pidfiles[i], []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(cgroup, "cgroup.subtree_control"), []byte("+cpu"), 0o644); err != nil {
				return err
			}
		}
		q, err := url.ParseQuery(p.RawQuery)
		if err != nil {
			return err
		}
		for key, values := range q {
			for _, value := range values {
				if err := os.WriteFile(filepath.Join(p.Path, key), []byte(value), 0o644); err != nil {
					return err
				}
			}
		}
	}
	opts.PidFile = strings.Join(pidfiles, ",")
	if opts.Debug {
		log.Debugf(cmd.OutOrStderr(), "RunE: version: %s", cmd.Root().Version)
	}
	return lobby.Run(cmd.Context(), &opts, cmd.OutOrStdout(), cmd.OutOrStderr())
}
