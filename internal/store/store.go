package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jin-take/SplitAgents/internal/domain"
)

type Store struct{ Root string }

func New() (*Store, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(h, ".splitagents")
	if err := os.MkdirAll(filepath.Join(root, "rooms"), 0o700); err != nil {
		return nil, err
	}
	return &Store{Root: root}, nil
}

func (s *Store) roomDir(id string) string { return filepath.Join(s.Root, "rooms", id) }
func (s *Store) paneLog(id string, pane int) string {
	return filepath.Join(s.roomDir(id), fmt.Sprintf("pane-%d.jsonl", pane))
}
func (s *Store) paneSummary(id string, pane int) string {
	return filepath.Join(s.roomDir(id), fmt.Sprintf("pane-%d-summary.md", pane))
}

func (s *Store) CreateRoom(title, goal string) (domain.Room, error) {
	now := time.Now()
	id := fmt.Sprintf("rm_%s", now.Format("20060102_150405.000000"))
	r := domain.Room{ID: id, Title: title, Goal: goal, PaneCount: 1, CreatedAt: now, UpdatedAt: now}
	if err := os.MkdirAll(s.roomDir(id), 0o700); err != nil {
		return r, err
	}
	return r, s.SaveRoom(r)
}

func (s *Store) SaveRoom(r domain.Room) error {
	r.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.roomDir(r.ID), "room.json"), append(b, '\n'), 0o600)
}

func (s *Store) LoadRoom(id string) (domain.Room, error) {
	var r domain.Room
	b, err := os.ReadFile(filepath.Join(s.roomDir(id), "room.json"))
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return r, err
}

func (s *Store) ListRooms(limit int) ([]domain.Room, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "rooms"))
	if err != nil {
		return nil, err
	}
	rooms := make([]domain.Room, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		r, err := s.LoadRoom(e.Name())
		if err == nil {
			rooms = append(rooms, r)
		}
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].UpdatedAt.After(rooms[j].UpdatedAt) })
	if limit > 0 && len(rooms) > limit {
		rooms = rooms[:limit]
	}
	return rooms, nil
}

func (s *Store) AppendMessage(roomID string, pane int, m domain.Message) error {
	f, err := os.OpenFile(s.paneLog(roomID, pane), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func (s *Store) ReadMessages(roomID string, pane, limit int) ([]domain.Message, error) {
	f, err := os.Open(s.paneLog(roomID, pane))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var all []domain.Message
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 2*1024*1024)
	for sc.Scan() {
		var m domain.Message
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			all = append(all, m)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, nil
}

func (s *Store) WriteSummary(roomID string, pane int, text string) error {
	return os.WriteFile(s.paneSummary(roomID, pane), []byte(strings.TrimSpace(text)+"\n"), 0o600)
}
func (s *Store) ReadSummary(roomID string, pane int) string {
	b, err := os.ReadFile(s.paneSummary(roomID, pane))
	if err != nil {
		return ""
	}
	return string(b)
}
