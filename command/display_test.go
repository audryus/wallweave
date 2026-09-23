package command

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/audryus/wallweave/db"
)

func newTestCommander(t *testing.T) *Commander {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	c := NewCommander(database)

	// seed fixo — evita depender de hyprctl nos testes de handler
	_, err = database.DB.Exec(
		`INSERT INTO displays (name, mirror_of, video, timer, seed, width, height, theme)
		 VALUES ('TEST-1', '', 0, 60, 1, 1920, 1080, 0)`,
	)
	if err != nil {
		t.Fatalf("seed display: %v", err)
	}

	return c
}

func decodeDisplays(t *testing.T, message string) []Display {
	t.Helper()
	var displays []Display
	if err := json.Unmarshal([]byte(message), &displays); err != nil {
		t.Fatalf("decode displays: %v", err)
	}
	return displays
}

func TestGetMonitors(t *testing.T) {
	displays, err := getMonitors()
	if err != nil {
		t.Skipf("hyprctl indisponível: %v", err)
	}
	if len(displays) == 0 {
		t.Error("expected at least 1 monitor")
	}
	for _, d := range displays {
		if d.Name == "" {
			t.Error("expected monitor name")
		}
	}
}

func TestHandleDisplays(t *testing.T) {
	c := newTestCommander(t)

	res := c.Exec(Request{Cmd: "browse_displays"})
	if res.Type != "browse_displays" {
		t.Fatalf("Expected browse_displays response, got %v (%v)", res.Type, res.Message)
	}

	displays := decodeDisplays(t, res.Message)
	if len(displays) != 1 {
		t.Fatalf("expected 1 display, got %d", len(displays))
	}
	if displays[0].Name != "TEST-1" {
		t.Errorf("expected name TEST-1, got %q", displays[0].Name)
	}
}

func TestHandleDisplay(t *testing.T) {
	c := newTestCommander(t)

	res := c.Exec(Request{Cmd: "edit_display", Display: Display{
		Name:  "TEST-1",
		Theme: 1,
		Video: true,
		Timer: 5,
	}})
	if res.Type != "edit_display" {
		t.Fatalf("Expected edit_display response, got %v (%v)", res.Type, res.Message)
	}

	var updated Display
	if err := json.Unmarshal([]byte(res.Message), &updated); err != nil {
		t.Fatalf("decode display: %v", err)
	}
	if updated.Theme != 1 || !updated.Video || updated.Timer != 5 {
		t.Errorf("expected updated fields, got %+v", updated)
	}

	// persistido no banco?
	stored, ok, err := c.getDisplay("TEST-1")
	if err != nil || !ok {
		t.Fatalf("getDisplay: ok=%v err=%v", ok, err)
	}
	if stored.Theme != 1 || !stored.Video || stored.Timer != 5 {
		t.Errorf("expected persisted fields, got %+v", stored)
	}
}

func TestHandleDisplayEmptyName(t *testing.T) {
	c := newTestCommander(t)

	res := c.Exec(Request{Cmd: "edit_display", Display: Display{Theme: 1}})
	if res.Type != "error" {
		t.Errorf("expected error for empty name, got %v", res.Type)
	}
}

func TestUnknownCommand(t *testing.T) {
	c := newTestCommander(t)

	res := c.Exec(Request{Cmd: "nope"})
	if res.Type != "error" {
		t.Errorf("expected error for unknown cmd, got %v", res.Type)
	}
}

func TestHandleStatusPersists(t *testing.T) {
	c := newTestCommander(t)

	res := c.Exec(Request{Cmd: "get_status"})
	if res.Type != "get_status" {
		t.Fatalf("expected get_status, got %v (%v)", res.Type, res.Message)
	}

	stored, err := c.loadStatus()
	if err != nil {
		t.Fatalf("loadStatus: %v", err)
	}
	if stored.Label == "" || stored.Color == "" {
		t.Errorf("expected persisted status fields, got %+v", stored)
	}
}
