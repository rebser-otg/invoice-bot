package memory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rebser-otg/invoice-bot/memory"
)

func TestLoad_EmptyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	m, err := memory.Load(filepath.Join(dir, "memory.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Contains("any-id") {
		t.Error("expected empty memory to not contain any ID")
	}
}

func TestMemory_AddAndContains(t *testing.T) {
	dir := t.TempDir()
	m, _ := memory.Load(filepath.Join(dir, "memory.json"))
	m.Add("msg-1")
	m.Add("msg-2")

	if !m.Contains("msg-1") {
		t.Error("expected msg-1 to be present")
	}
	if !m.Contains("msg-2") {
		t.Error("expected msg-2 to be present")
	}
	if m.Contains("msg-3") {
		t.Error("expected msg-3 to be absent")
	}
}

func TestMemory_SaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	m, _ := memory.Load(path)
	m.Add("msg-abc")
	if err := m.Save(path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	m2, err := memory.Load(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if !m2.Contains("msg-abc") {
		t.Error("expected msg-abc to persist after save/reload")
	}
}

func TestMemory_AtomicSave_NoTempFileLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := memory.Load(path)
	m.Add("msg-1")
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file after atomic save, got %d: %v", len(entries), entries)
	}
}
