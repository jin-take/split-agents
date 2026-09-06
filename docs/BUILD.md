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

## Pane focus

SplitAgents assigns a logical Pane number to each agent process. Move focus directly with:

```text
/pane 1
/pane 2
/pane 3
```

The command resolves the tmux pane that was started with the matching `--pane N` argument, so it does not depend on the user's tmux pane-base-index setting.

Standard tmux focus controls continue to work as well:

```text
Ctrl+b then Arrow Key
Ctrl+b then q then pane number
```

### macOS `Command + 1/2/3`

A CLI process cannot receive `Command` shortcuts that are consumed by the terminal emulator. Apple Terminal.app reserves `Command + number` for tab selection, so SplitAgents cannot override that shortcut from inside tmux.

For an exact `Command + 1/2/3` workflow, configure the terminal or a macOS key-remapping tool to send the following text plus Enter to the active SplitAgents pane:

```text
Command+1 -> /pane 1
Command+2 -> /pane 2
Command+3 -> /pane 3
```

Terminal emulators that support arbitrary key mappings can send those strings directly. When using Apple Terminal.app, an external key remapper such as Karabiner-Elements or Hammerspoon is required for the exact `Command + number` mapping.

### macOS `Command + Shift + 1/2/3`

The same approach can be used for Pane creation:

```text
Command+Shift+1 -> /split 1
Command+Shift+2 -> /split 2
Command+Shift+3 -> /split 3
```

## Environment

The application automatically loads `.env` from the current working directory at startup. The original `.env` path is propagated to newly opened Terminal windows and tmux panes with `SPLITAGENTS_ENV_FILE`.

```dotenv
OPENAI_API_KEY="YOUR_KEY"
```

| Variable | Required | Purpose |
|---|---:|---|
| `OPENAI_API_KEY` | yes | OpenAI Responses API authentication |
| `SPLITAGENTS_ENV_FILE` | internal | Absolute path propagated to child Terminal/tmux processes |

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
