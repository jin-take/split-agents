package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jin-take/SplitAgents/internal/compiler"
	"github.com/jin-take/SplitAgents/internal/domain"
	"github.com/jin-take/SplitAgents/internal/openai"
	"github.com/jin-take/SplitAgents/internal/store"
)

type App struct {
	Store    *store.Store
	AI       *openai.Client
	Compiler *compiler.Compiler
	Exe      string
}

func New() (*App, error) {
	s, err := store.New()
	if err != nil {
		return nil, err
	}
	ai, err := openai.New()
	if err != nil {
		return nil, err
	}
	exe, _ := os.Executable()
	return &App{Store: s, AI: ai, Compiler: compiler.New(ai), Exe: exe}, nil
}

func (a *App) Start(ctx context.Context) error {
	rooms, err := a.Store.ListRooms(10)
	if err != nil {
		return err
	}
	fmt.Println("\nSplitAgents — Recent Rooms\n\n[0] New")
	for i, r := range rooms {
		fmt.Printf("[%d] %s  (%s)\n", i+1, r.Title, humanAgo(r.UpdatedAt))
	}
	fmt.Print("\n> ")
	in, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(in))
	if err != nil || n < 0 || n > len(rooms) {
		return fmt.Errorf("invalid selection")
	}
	var room domain.Room
	if n == 0 {
		fmt.Print("Room goal > ")
		goal, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		goal = strings.TrimSpace(goal)
		if goal == "" {
			return fmt.Errorf("room goal is required")
		}
		plan := a.Compiler.Plan(ctx, goal, true)
		title := plan.Title
		if title == "" {
			title = "New Room"
		}
		room, err = a.Store.CreateRoom(title, goal)
		if err != nil {
			return err
		}
	} else {
		room = rooms[n-1]
	}
	return a.openNewTerminal(room.ID)
}

func (a *App) OpenRoom(ctx context.Context, id string) error {
	r, err := a.Store.LoadRoom(id)
	if err != nil {
		return err
	}
	if os.Getenv("TMUX") == "" && has("tmux") {
		name := "splitagents-" + safe(r.ID)
		cmd := exec.Command("tmux", "new-session", "-A", "-s", name, a.Exe, "pane", "run", "--room", id, "--pane", "1")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return a.RunPane(ctx, id, 1)
}

func (a *App) RunPane(ctx context.Context, roomID string, pane int) error {
	r, err := a.Store.LoadRoom(roomID)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%s\nRoom: %s | Pane: %d/%d\nCommands: /split 1|2|3, /pane N, /status, /quit\n\n", r.Title, r.ID, pane, r.PaneCount)
	for {
		fmt.Printf("P%d > ", pane)
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		q := strings.TrimSpace(line)
		if q == "" {
			continue
		}
		if q == "/quit" {
			return nil
		}
		if strings.HasPrefix(q, "/split ") {
			n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(q, "/split ")))
			if err := a.split(roomID, n); err != nil {
				fmt.Println("split:", err)
			}
			continue
		}
		if q == "/status" {
			fmt.Printf("room=%s pane=%d logs=%s\n", roomID, pane, a.Store.Root)
			continue
		}
		if strings.HasPrefix(q, "/pane ") {
			fmt.Println("Use tmux focus (or click the pane) to switch active panes.")
			continue
		}
		plan := a.Compiler.Plan(ctx, q, false)
		msgs, _ := a.Store.ReadMessages(roomID, pane, plan.RecentMessages)
		summary := a.Store.ReadSummary(roomID, pane)
		input := buildInput(r, summary, msgs, q)
		_ = a.Store.AppendMessage(roomID, pane, domain.Message{At: time.Now(), Role: "user", Content: q})
		fmt.Printf("[%s/%s] ", plan.Model, plan.Reasoning)
		text, usage, err := a.AI.Stream(ctx, plan.Model, plan.Reasoning, systemPrompt(r), input, 5000, func(d string) { fmt.Print(d) })
		fmt.Println()
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		_ = a.Store.AppendMessage(roomID, pane, domain.Message{At: time.Now(), Role: "assistant", Content: text, Model: plan.Model, Input: usage.InputTokens, Output: usage.OutputTokens, Reasoning: usage.ReasoningTokens})
		a.maybeCompress(ctx, roomID, pane)
	}
}

func (a *App) split(roomID string, n int) error {
	if n < 1 || n > 3 {
		return fmt.Errorf("pane count must be 1..3")
	}
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("tmux is required for visual split panes")
	}
	r, err := a.Store.LoadRoom(roomID)
	if err != nil {
		return err
	}
	if n <= r.PaneCount {
		return nil
	}
	for p := r.PaneCount + 1; p <= n; p++ {
		args := []string{"split-window"}
		if p == 2 {
			args = append(args, "-h")
		} else {
			args = append(args, "-v")
		}
		args = append(args, a.Exe, "pane", "run", "--room", roomID, "--pane", strconv.Itoa(p))
		if err := exec.Command("tmux", args...).Run(); err != nil {
			return err
		}
	}
	_ = exec.Command("tmux", "select-layout", "tiled").Run()
	r.PaneCount = n
	return a.Store.SaveRoom(r)
}

func (a *App) maybeCompress(ctx context.Context, roomID string, pane int) {
	msgs, _ := a.Store.ReadMessages(roomID, pane, 0)
	if len(msgs) < 16 || len(msgs)%8 != 0 {
		return
	}
	var b strings.Builder
	for _, m := range msgs[:len(msgs)-6] {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
	}
	s, _, err := a.AI.Complete(ctx, "gpt-5.6-luna", "none", "Compress this conversation into durable facts, decisions, constraints, unresolved questions, and useful context. Be concise. Do not add commentary.", b.String(), 700)
	if err == nil {
		_ = a.Store.WriteSummary(roomID, pane, s)
	}
}

func buildInput(r domain.Room, summary string, msgs []domain.Message, current string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ROOM GOAL:\n%s\n", r.Goal)
	if summary != "" {
		fmt.Fprintf(&b, "\nCOMPRESSED PRIOR CONTEXT:\n%s\n", summary)
	}
	if len(msgs) > 0 {
		b.WriteString("\nRECENT MESSAGES:\n")
		for _, m := range msgs {
			fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
		}
	}
	fmt.Fprintf(&b, "\nCURRENT USER REQUEST:\n%s", current)
	return b.String()
}
func systemPrompt(r domain.Room) string {
	return "You are working inside one SplitAgents room. Stay focused on the room goal. Give the useful answer directly. Do not mention internal routing, token optimization, or hidden reasoning unless asked."
}
func (a *App) openNewTerminal(id string) error {
	if runtime.GOOS != "darwin" {
		return a.OpenRoom(context.Background(), id)
	}
	script := fmt.Sprintf(`tell application "Terminal" to do script %q`, a.Exe+" room open "+id)
	return exec.Command("osascript", "-e", script).Run()
}
func has(name string) bool { _, err := exec.LookPath(name); return err == nil }
func safe(s string) string { return strings.NewReplacer("_", "-", ".", "-").Replace(s) }
func humanAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return t.Format("Jan 2")
}
