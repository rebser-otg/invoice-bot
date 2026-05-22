package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Memory tracks which Gmail message IDs have been forwarded.
type Memory struct {
	Forwarded []string `json:"forwarded"`
	index     map[string]struct{}
}

// Load reads memory.json from path. Returns an empty Memory if the file does not exist.
func Load(path string) (*Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Memory{index: make(map[string]struct{})}, nil
		}
		return nil, fmt.Errorf("reading memory: %w", err)
	}
	var m Memory
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing memory: %w", err)
	}
	m.index = make(map[string]struct{}, len(m.Forwarded))
	for _, id := range m.Forwarded {
		m.index[id] = struct{}{}
	}
	return &m, nil
}

// Contains reports whether id has been forwarded.
func (m *Memory) Contains(id string) bool {
	_, ok := m.index[id]
	return ok
}

// Add records id as forwarded. No-op if already present.
func (m *Memory) Add(id string) {
	if !m.Contains(id) {
		m.Forwarded = append(m.Forwarded, id)
		m.index[id] = struct{}{}
	}
}

// Len returns the number of forwarded IDs.
func (m *Memory) Len() int {
	return len(m.Forwarded)
}

// Save writes memory to path atomically (write to temp, then rename).
func (m *Memory) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling memory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
