# ccgrass design

Turn Claude Code session logs into a daily "learning log" contribution graph.

## Pipeline

```
~/.claude/projects/*/*.jsonl
  → parse events (timestamp, session, tool inputs)
  → classify sessions (does it touch a tracked path?)
  → active minutes per day
  → merge into the store (data/days.json)
  → render (SVG heatmap, JSON for the portfolio site)
```

## Input

Each line of a transcript is one JSON object. Fields used:

- `type`: only `user` and `assistant` events count as activity.
- `timestamp`: RFC 3339 in UTC.
- `sessionId`
- `message.content[]` of `assistant` events: `tool_use` items carry `input.file_path` (Read, Edit, Write) or `input.command` (Bash).

Unknown event types and fields are ignored, because the format is not a public API and changes between versions.

## Active time

- Merge the events of all tracked sessions into one timeline, so parallel sessions are not counted twice.
- Count the gap between two consecutive events when it is at most `--break` (default 15m). A longer gap is a break and counts nothing.
- Days are split at midnight in `--tz` (default Asia/Tokyo). A gap that crosses midnight is split between the two days.

## Classification

A session is tracked when any tool use touches one of the `--path` prefixes (for example `~/code/oss`, `~/code/zenn`, `~/code/ccgrass`): `file_path` starting with the prefix, or a Bash `command` containing it. The whole session counts. Sessions are launched from `~/code`, so `cwd` cannot be used.

## Store

Claude Code deletes transcripts after 30 days (`cleanupPeriodDays`), so the tool keeps its own aggregates:

```json
{"days": {"2026-10-08": {"minutes": 95, "repos": ["oss", "ccgrass"]}}}
```

- Only aggregates are stored. Never store message content.
- Each run recomputes the days that still have transcripts and keeps older days as they are. If a recomputed day is lower than the stored value (part of it was already deleted), keep the larger one.

## Grass

A day is green when its minutes are at least `--min-minutes` (default 5). Shades by minutes are a later option.

## Milestones

1. Parse transcripts and print active minutes per day (no classification).
2. Classification by `--path`.
3. The store and merge rules.
4. SVG heatmap.
5. Portfolio integration (eyoshioka.github.io reads the JSON or SVG at build time).
6. Daily run (launchd or a scheduled task).

## Testing

Golden-file tests with small synthetic JSONL fixtures under `testdata/`. Never copy real transcripts into the repo.

## Open questions

- How the data reaches the site: commit the JSON to the portfolio repo, or publish it from this repo.
- Whether a break should end at the last event or add a fixed tail (for example, time spent reading the last answer).
