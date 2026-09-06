# Functional Specification

## 1. Goal

SplitAgents provides a terminal-first AI workspace where one Room represents one problem and up to three Panes execute independent task threads against that problem. The primary design goal is answer quality per token, not maximum reasoning on every request.

## 2. CLI

### `chatgpt run start`

Shows `[0] New` plus the 10 most recently updated Rooms. Creating a Room asks for the Room goal and generates a short English title using the low-cost planner. Selecting/creating a Room opens a new macOS Terminal tab.

### `chatgpt room open <room-id>`

Opens the Room. When tmux is available, a dedicated `splitagents-*` tmux session is created/reused.

### Pane commands

- `/split 1|2|3`: increase visual Pane count; maximum 3.
- `/status`: show current room/pane and storage root.
- `/quit`: leave the Pane.

Each Pane is an independent process and independent conversation log. All Panes share only the Room goal by default.

## 3. Persistence

No DB is required.

```text
~/.splitagents/rooms/<room-id>/
  room.json
  pane-1.jsonl
  pane-1-summary.md
  pane-2.jsonl
  pane-2-summary.md
  pane-3.jsonl
  pane-3-summary.md
```

`room.json` stores title, goal, timestamps and Pane count. `pane-N.jsonl` is append-only.

## 4. Token Compiler

The compiler does not solve the user's request. It decides how the request should be solved.

### Fast path

Small, non-complex prompts skip the planner API call and use a heuristic profile, typically Luna with no reasoning and only four recent messages.

### Planned path

More complex prompts use `gpt-5.6-luna` with low reasoning to return a compact execution plan:

- task summary
- complexity
- execution model
- reasoning effort
- context keywords
- requested subagent count (0-3)
- recent-message window

The current v0.1 records subagent intent but does not yet auto-spawn subagents. Visual Panes are explicitly user-controlled.

## 5. Context policy

Execution context contains only:

1. Room goal
2. compressed prior Pane context, when available
3. a small recent-message window selected by the compiler
4. current user request

After the Pane history grows, older messages are periodically summarized using Luna with no reasoning. Full history remains on disk and is not deleted.

## 6. Model escalation

Default intent:

- simple: Luna / none
- normal: Terra / low
- complex: Sol / medium

The planner may lower or raise this when useful. Strong models should receive a smaller, cleaner context rather than the entire Room history.

## 7. Out of scope for v0.1

- database persistence
- browser UI
- automatic GitHub/Web/MCP tools
- autonomous subagent spawning
- shared mutable memory across Panes
- cross-Pane summary import
- global macOS shortcut registration

These are intentionally deferred so token economics can be measured before the runtime becomes more autonomous.
