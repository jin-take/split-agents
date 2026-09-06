# Build and Operations

## Requirements

- macOS for the current new-Terminal-tab launcher
- Go 1.24+
- OpenAI API key
- tmux recommended for actual visual Pane splitting

## Install

```bash
brew install go tmux
```

Create a `.env` file in the repository root:

```bash
cat > .env <<'EOF'
OPENAI_API_KEY="YOUR_KEY"
EOF
```

Then install:

```bash
make install
```

Ensure `$(go env GOPATH)/bin` is on your PATH:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Run SplitAgents from the repository root so the local `.env` is loaded:

```bash
cd /path/to/split-agents
chatgpt run start
```

If `OPENAI_API_KEY` is already exported in the shell, the existing process environment takes precedence over the value in `.env`.

## Development

From the repository root:

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

The application automatically loads `.env` from the current working directory at startup.

```dotenv
OPENAI_API_KEY="YOUR_KEY"
```

| Variable | Required | Purpose |
|---|---:|---|
| `OPENAI_API_KEY` | yes | OpenAI Responses API authentication |

`.env` is ignored by Git and must never be committed.

## OpenAI request behavior

Execution responses use the Responses API with streaming enabled. Planner and compression calls are non-streaming and intentionally capped to small outputs.

The default routing policy is code-owned so it can be benchmarked and changed without migrating persisted Room data.

## Operational notes

- The API key is never written to Room logs.
- Existing shell environment variables override `.env` values.
- Room and Pane files are created with user-only permissions where applicable.
- JSONL logs are append-only.
- Pane summaries are derived cache and can be deleted/rebuilt later.
- The current implementation loads `.env` from the process working directory, so start `chatgpt` from the `split-agents` repository root when relying on the repository-local `.env`.
