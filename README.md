snap-gs is a cross-platform [Snapshot VR](https://snapshotvr.com/) game-server
supervisor for Windows and Linux. It ensures healthy game lobbies for dedicated
servers and writes compressed match result JSON to disk for further processing.

snap-gs simplifies operations for both dedicated servers and temporary lobbies:

* Idle lobby timeout. Automatically restart lobby when unused or last _human_ player leaves.
* Collect match results. Final match JSON compressed and written to `--logdir` or `--matchdir`.
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
          --roomname string         lobby name
          --password string         lobby auth
          --specdir string          read desired lobby status here
          --statdir string          write current lobby status here
          --matchdir string         write compressed match JSON here
          --logdir string           write compressed lobby logs here
          --exe string              path to Snapshot VR executable
          --timeout duration        timeout lobby when no players join (default 15h0m0s)
          --admintimeout duration   timeout lobby when admin delays match start (default 15m0s)
          --minuptime duration      min time lobby must run (default 15s)
          --maxfails int            max times lobby may fail (default 3)
          --maxidles int            max times lobby may idle (default -1)
      -h, --help                    help for lobby

    Global Flags:
          --debug   enable debug output

# Examples

### --logdir

    $ snap-gs lobby --roomname=snap-gs --logdir=logs

    [  96]  ./logs
    |-- [290K]  2022-03-21T20_30_00Z.lobby.log.gz
    |-- [7.9K]  2022-03-21T20_36_17Z.match.json.gz
    |-- [7.3K]  2022-03-21T20_43_17Z.match.json.gz
    |-- [ 10K]  2022-03-21T20_53_22Z.match.json.gz
    [...]

### --logdir and --matchdir

    $ snap-gs lobby --roomname=snap-gs --logdir=logs --matchdir=matches

    [  96]  ./logs
    `-- [290K]  2022-03-21T20_30_00Z.lobby.log.gz
    [1.2K]  ./matches
    |-- [7.9K]  2022-03-21T20_36_17Z.match.json.gz
    |-- [7.3K]  2022-03-21T20_43_17Z.match.json.gz
    |-- [ 10K]  2022-03-21T20_53_22Z.match.json.gz
    [...]

### --matchdir and --debug

    $ snap-gs lobby --roomname=snap-gs --matchdir=matches --debug

    D: 0000.0004 Lobby.runc: c=snapshot_server -nographics -batchmode --roomname snap-gs
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

    $ snap-gs lobby --debug --maxfails=0 --maxidles=0 --roomname=snap-gs --logdir=logs --matchdir=matches \
        --exe="bash,-c,cat < out.log & cat < err.log >&2 & wait,bash"
