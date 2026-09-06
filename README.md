# SplitAgents

Terminal-first AI workspace focused on token efficiency.

## Concept

- **Room**: one problem / objective.
- **Pane**: an independent task stream inside the same Room. Maximum 3.
- **Token Compiler**: plans, prunes context, chooses model/reasoning, and escalates only when necessary.
- **Storage**: local JSON/JSONL under `~/.splitagents`; no database.

## Quick start

Requirements: Go 1.24+, `OPENAI_API_KEY`, and optional `tmux` for visual split panes.

```bash
export OPENAI_API_KEY="..."
make install
chatgpt run start
```

`make install` installs the binary as `chatgpt` into `$(go env GOPATH)/bin`.

The launcher shows up to 10 recent Rooms:

```text
[0] New
[1] SVL Contract Investigation
[2] Arbitrage Architecture Design
...
```

Selecting a Room opens it in a new macOS Terminal tab. If `tmux` is installed, the Room runs as a tmux session.

Inside a Room:

```text
/split 2
/split 3
/status
/quit
```

Each Pane gets its own `pane-N.jsonl` conversation history and compressed summary.

## Token-efficiency strategy

1. Very small/simple prompts skip the planner entirely.
2. Non-trivial prompts go through a cheap `gpt-5.6-luna` planner.
3. The planner selects Luna/Terra/Sol, reasoning effort, recent-message count, and at most 3 subagents.
4. Only recent messages plus a compressed Pane summary are sent to the execution model.
5. Old context is periodically compressed with Luna at `reasoning: none`.
6. API usage is persisted alongside assistant messages for later optimization.

See `docs/SPEC.md`, `docs/ARCHITECTURE.md`, and `docs/BUILD.md`.

## Status

v0.1 is intentionally small: terminal chat, Rooms, independent Panes, local logs, model/reasoning routing, context compression, streaming Responses API output, and tmux split support. Tool execution and automatic subagent orchestration are next-stage work.
