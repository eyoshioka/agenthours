# agenthours

A contribution graph of the hours you spend working with coding agents.

Most usage tools for coding agents count tokens or cost: how much the agent
worked. agenthours measures your active time with the agent instead, and
renders it as a calendar heatmap you can embed in a README, a GitHub profile
or a personal site.

> **Status: early development.** The CLI prints minutes per day for all
> Claude Code sessions; filtering, history and the SVG are not there yet. See
> [DESIGN.md](DESIGN.md) for the plan and the milestones.

## Usage

```sh
go install github.com/eyoshioka/agenthours@latest
agenthours
```

```
2026-10-08	547
2026-10-09	149
```

One line per day: the date, a tab, and the active minutes (rounded down).

| Flag | Default | Meaning |
|---|---|---|
| `-dir` | `~/.claude/projects` | Claude Code projects directory |
| `-break` | `15m` | Longest gap between messages that still counts as active |
| `-tz` | `Asia/Tokyo` | Time zone for day boundaries |

## How it works

- Reads the session logs your agent already keeps on disk. Claude Code
  (`~/.claude/projects/*/*.jsonl`) is the first supported agent.
- Counts the gap between two consecutive messages as active time when it is
  at most 15 minutes; a longer gap is a break. Parallel sessions are not
  counted twice.
- Optionally counts only sessions that touch chosen paths, such as your
  open-source or learning repositories.
- Stores only daily totals. Message content never leaves the logs.

## Planned

- Filtering by touched paths
- A local store that keeps history after the agent deletes old logs
- SVG and JSON output for embedding

## Development

Requires Go (see `go.mod`) and [mise](https://mise.jdx.dev/) for the pinned
golangci-lint.

```sh
mise install
make check   # lint, tests and the 80% coverage gate
```

## Disclaimer

agenthours is an independent project. It is not affiliated with, endorsed by
or sponsored by Anthropic or any other agent vendor.

## License

[MIT](LICENSE)
