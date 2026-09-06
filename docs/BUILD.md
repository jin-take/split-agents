# Build and Operations

## Requirements

- macOS for the current new-Terminal-tab launcher
- Go 1.24+
- OpenAI API key
- tmux recommended for actual visual Pane splitting

## Install

```bash
brew install go tmux
export OPENAI_API_KEY="YOUR_KEY"
make install
```

Ensure `$(go env GOPATH)/bin` is on your PATH:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Then run:

```bash
chatgpt run start
```

## Development

```bash
go run ./cmd/chatgpt run start
go test ./...
go vet ./...
```

## Local data

All runtime state is under:

```text
~/.splitagents
```

To inspect a Pane log:

```bash
jq . ~/.splitagents/rooms/<room-id>/pane-1.jsonl
```

To reset all local state:

```bash
rm -rf ~/.splitagents
```

## Pane splitting

Inside a Room:

```text
/split 2
/split 3
```

This calls tmux `split-window` and starts a separate `chatgpt pane run` process for each new Pane.

### macOS `Command + Shift + 1/2/3`

A CLI process cannot reliably capture macOS Command-key shortcuts because Terminal.app consumes them first. For v0.1, bind the desired Terminal/iTerm shortcut to send these strings to the active Pane:

```text
/split 1\n
/split 2\n
/split 3\n
```

The runtime remains terminal-agnostic while preserving the requested shortcut UX through the terminal emulator's key mapping.

## Environment

| Variable | Required | Purpose |
|---|---:|---|
| `OPENAI_API_KEY` | yes | OpenAI Responses API authentication |

## OpenAI request behavior

Execution responses use the Responses API with streaming enabled. Planner and compression calls are non-streaming and intentionally capped to small outputs.

The default routing policy is code-owned so it can be benchmarked and changed without migrating persisted Room data.

## Operational notes

- The API key is never written to Room logs.
- Room and Pane files are created with user-only permissions where applicable.
- JSONL logs are append-only.
- Pane summaries are derived cache and can be deleted/rebuilt later.
- The current implementation requires an API key even to open the launcher because the client is initialized at startup; moving initialization to first API use is a small future hardening item.
