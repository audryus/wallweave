package command

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/audryus/wallweave/db"
)

// newTestCommander creates a Commander backed by a temporary database with
// one seeded display ("TEST-1"). Tests use it so handlers can be exercised
// without a real hyprctl.
func newTestCommander(t *testing.T) *Commander {
	// Mark this as a helper so test failures point at the caller.
	t.Helper()

	// Open a fresh database in a temporary folder (removed after the test).
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// Close the database when the test finishes.
	t.Cleanup(func() { database.Close() })

	// Create the commander under test.
	c := NewCommander(database)

	// Fixed seed — avoids depending on hyprctl in the handler tests.
	_, err = database.DB.Exec(
		`INSERT INTO displays (name, mirror_of, video, timer, seed, width, height, theme)
		 VALUES ('TEST-1', '', 0, 60, 1, 1920, 1080, 0)`,
	)
	if err != nil {
		t.Fatalf("seed display: %v", err)
	}

	return c
}

// decodeDisplays decodes the JSON message of a browse_displays response
// into a slice of Display structs.
func decodeDisplays(t *testing.T, message string) []Display {
	t.Helper()
	var displays []Display
	if err := json.Unmarshal([]byte(message), &displays); err != nil {
		t.Fatalf("decode displays: %v", err)
	}
	return displays
}

// TestGetMonitors checks that hyprctl can list the connected monitors.
// It is skipped when hyprctl is not available on the machine.
func TestGetMonitors(t *testing.T) {
	// Ask hyprctl for the monitors.
	displays, err := getMonitors()
	if err != nil {
		// Skip the test when hyprctl is unavailable.
		t.Skipf("hyprctl unavailable: %v", err)
	}
	// At least one monitor must be reported.
	if len(displays) == 0 {
		t.Error("expected at least 1 monitor")
	}
	// Every monitor must have a non-empty name.
	for _, d := range displays {
		if d.Name == "" {
			t.Error("expected monitor name")
		}
	}
}

// TestHandleDisplays checks the "browse_displays" command: it must return
// the single display that was seeded for this test.
func TestHandleDisplays(t *testing.T) {
	// Create the commander with the seeded display.
	c := newTestCommander(t)

	// Run the browse command.
	res := c.Exec(Request{Cmd: "browse_displays"})
	if res.Type != "browse_displays" {
		t.Fatalf("Expected browse_displays response, got %v (%v)", res.Type, res.Message)
	}

	// Decode and check that exactly one display comes back.
	displays := decodeDisplays(t, res.Message)
	if len(displays) != 1 {
		t.Fatalf("expected 1 display, got %d", len(displays))
	}
	if displays[0].Name != "TEST-1" {
		t.Errorf("expected name TEST-1, got %q", displays[0].Name)
	}
}

// TestHandleDisplay checks the "edit_display" command: the updated fields
// must come back in the response AND be persisted in the database.
func TestHandleDisplay(t *testing.T) {
	// Create the commander with the seeded display.
	c := newTestCommander(t)

	// Edit the display: set theme 1, enable video, timer 5 seconds.
	res := c.Exec(Request{Cmd: "edit_display", Display: Display{
		Name:  "TEST-1",
		Theme: 1,
		Video: true,
		Timer: 5,
	}})
	if res.Type != "edit_display" {
		t.Fatalf("Expected edit_display response, got %v (%v)", res.Type, res.Message)
	}

	// The response must contain the updated values.
	var updated Display
	if err := json.Unmarshal([]byte(res.Message), &updated); err != nil {
		t.Fatalf("decode display: %v", err)
	}
	if updated.Theme != 1 || !updated.Video || updated.Timer != 5 {
		t.Errorf("expected updated fields, got %+v", updated)
	}

	// Was it persisted in the database?
	stored, ok, err := c.getDisplay("TEST-1")
	if err != nil || !ok {
		t.Fatalf("getDisplay: ok=%v err=%v", ok, err)
	}
	if stored.Theme != 1 || !stored.Video || stored.Timer != 5 {
		t.Errorf("expected persisted fields, got %+v", stored)
	}
}

// TestHandleDisplayEmptyName checks that editing a display without a name
// returns an error.
func TestHandleDisplayEmptyName(t *testing.T) {
	// Create the commander with the seeded display.
	c := newTestCommander(t)

	// An edit with no display name must fail with a stable code.
	res := c.Exec(Request{Cmd: "edit_display", Display: Display{Theme: 1}})
	if res.Type != "error" {
		t.Errorf("expected error for empty name, got %v", res.Type)
	}
	if res.Code != "display_empty" {
		t.Errorf("expected code display_empty, got %q", res.Code)
	}
}

// TestUnknownCommand checks that an unknown command name returns an error.
func TestUnknownCommand(t *testing.T) {
	// Create the commander with the seeded display.
	c := newTestCommander(t)

	// A command that was never registered must fail with a stable code.
	res := c.Exec(Request{Cmd: "nope"})
	if res.Type != "error" {
		t.Errorf("expected error for unknown cmd, got %v", res.Type)
	}
	if res.Code != "unknown_command" {
		t.Errorf("expected code unknown_command, got %q", res.Code)
	}
}

// TestHandleStatusPersists checks that "get_status" probes the system and
// saves the result: after the call, loadStatus must return a non-empty
// label and color.
func TestHandleStatusPersists(t *testing.T) {
	// Create the commander with the seeded display.
	c := newTestCommander(t)

	// Run the status command.
	res := c.Exec(Request{Cmd: "get_status"})
	if res.Type != "get_status" {
		t.Fatalf("expected get_status, got %v (%v)", res.Type, res.Message)
	}

	// The status must have been saved in the database with a stable code
	// for the badge label (never an English sentence).
	stored, err := c.loadStatus()
	if err != nil {
		t.Fatalf("loadStatus: %v", err)
	}
	if stored.Label == "" || stored.Color == "" {
		t.Errorf("expected persisted status fields, got %+v", stored)
	}
	if stored.Label != "running" && stored.Label != "degraded" {
		t.Errorf("expected label code running|degraded, got %q", stored.Label)
	}
}
