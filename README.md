snap-gs is a cross-platform [Snapshot VR](https://snapshotvr.com/) game-server
supervisor for Windows and Linux. It ensures healthy game lobbies for dedicated
servers and writes compressed match result JSON to disk for further processing.

snap-gs simplifies operations for both dedicated servers and temporary lobbies:

* Collect match results. Final match JSON compressed and written to `--logdir`.
* Idle lobby timeout. Automatically restart lobby when unused or last _human_ player leaves.
* Idempotent match results. `@timestamp` parsed from match ID and added to match filename/JSON.
* `snapshot_server` log files. Every lobby process writes a new compressed log file to `--logdir`.
* Cleaner log files. Drop redundant lines/JSON and add a sub-ms timestamp to every line.

snap-gs derives lobby state from `snapshot_server` log lines, primarily those
originating from BOLT netcode. It assumes single-line JSON blobs are match
updates and uses heuristics to write the final update to disk.

snap-gs correctness relies entirely on the existing `snapshot_server` log stream.
Some lobby state is updated by redundant inputs, but most is not, and logs are
subject to change at whim. If you find the goals of this library valuable,
please petition [#feedback](https://discord.com/channels/605073897372647435/605074079497715712)
(Snapshot VR Discord) to add a robust and well-formed method to collect match
JSON and learn lobby state.

snap-gs is a standalone CLI binary by default. The `public/cmd` package
implementing the CLI is both directly extendable (new subcommands) and
embeddable (subcommand within another cobra-based CLI). The underlying lobby
management `public/lobby` package is also importable.

# Quickstart

Recommended for most cases:

    $ go run ./cmd/snap-gs lobby --help

Alternative if `PATH` includes `GOBIN`:

    $ go install ./cmd/snap-gs
    $ snap-gs lobby --help

Usage looks like this:

    Usage:
      snap-gs lobby [flags]

    Flags:
          --session string          set lobby name
          --password string         set lobby auth
          --flagdir string          read desired flags from <flagdir>
          --specdir string          read desired status from <specdir>
          --statdir string          write current status to <statdir>
          --logdir string           write logs and matches to <logdir>
          --pidfile string          write main[,busy,idle] <pidfile>
          --maxfails int            max fails before hard stop (default 3)
          --minuptime duration      min uptime before soft stop (default 5m0s)
          --admintimeout duration   timeout when admin delays match (default 15m0s)
          --timeout duration        timeout when no players join (default 15h0m0s)
          --listen string           bind local[,public,accel] ip:port
          --exe string              path to executable
      -h, --help                    help for lobby

    Global Flags:
          --debug   enable debug output

# Examples

### --logdir

    $ snap-gs lobby --session=snap-gs --logdir=logs

    [  96]  ./logs
    |-- [290K]  2022-03-21T20_30_00Z.lobby.log.gz
    |-- [7.9K]  2022-03-21T20_36_17Z.match.json.gz
    |-- [7.3K]  2022-03-21T20_43_17Z.match.json.gz
    |-- [ 10K]  2022-03-21T20_53_22Z.match.json.gz
    [...]

### --logdir and --matchdir

    $ snap-gs lobby --session=snap-gs --logdir=logs --matchdir=matches

    [  96]  ./logs
    `-- [290K]  2022-03-21T20_30_00Z.lobby.log.gz
    [1.2K]  ./matches
    |-- [7.9K]  2022-03-21T20_36_17Z.match.json.gz
    |-- [7.3K]  2022-03-21T20_43_17Z.match.json.gz
    |-- [ 10K]  2022-03-21T20_53_22Z.match.json.gz
    [...]

### --matchdir and --debug

    $ snap-gs lobby --session=snap-gs --matchdir=matches --debug

    D: 0000.0004 Lobby.runc: c=snapshot_server -nographics -batchmode --session snap-gs
    D: 0000.0005 Lobby.spooler: matchdir=matches
    1> 0000.0070 Mono path[0] = 'SnapshotVR_Data/Managed'
    1> 0000.0070 Mono config path = 'MonoBleedingEdge/etc'
    [...]
    D: 4823.6860 Lobby.filterbolt: players=0
    1> 4823.6854 -- BOLT -- Unregistered player: 1117
    1> 4823.6855 IdlePlayerManager Check Running
    D: 4823.6861 Lobby.spooler: done

    [1.2K]  ./matches
    |-- [7.9K]  2022-03-21T20_36_17Z.match.json.gz
    |-- [7.3K]  2022-03-21T20_43_17Z.match.json.gz
    |-- [ 10K]  2022-03-21T20_53_22Z.match.json.gz
    [...]

# Development

`--exe` supports a comma-separated list of arguments and `--maxfails=0` with
`--maxidles=0` disables automatic lobby restarts. This is useful when eg.
processing raw `snapshot_server` log files for testing:

    $ snap-gs lobby --debug --maxfails=0 --maxidles=0 --session=snap-gs --logdir=logs --matchdir=matches \
        --exe="bash,-c,cat < out.log & cat < err.log >&2 & wait,bash"
