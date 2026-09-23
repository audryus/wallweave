package command

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
)

// Display represents one monitor of the system, with its wallpaper
// settings: which library (theme) is selected, the rotation timer, whether
// videos are allowed, a random seed for shuffling, and the resolution.
type Display struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	MirrorOf string `json:"mirrorOf"`
	Video    bool   `json:"video"`
	Timer    int    `json:"timer"`
	Seed     int64  `json:"seed"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Theme    int    `json:"theme"`
}

// displayColumns lists every column of the displays table, in the same
// order used by the Scan functions below.
const displayColumns = `id, name, mirror_of, video, timer, seed, width, height, theme`

// registerDisplay registers the commands used to list and edit displays
// (monitors): "browse_displays" and "edit_display".
func (c *Commander) registerDisplay() {
	c.Register("browse_displays", c.browseDisplays)
	c.Register("edit_display", c.editDisplay)
}

// browseDisplays is the handler for the "browse_displays" command. Steps:
//  1. Load all displays from the database.
//  2. If the table is empty, detect the monitors and insert them (bootstrap).
//  3. Make sure a worker is running for every display that has a library
//     selected (theme != 0).
//  4. Return the list as JSON.
func (c *Commander) browseDisplays(req Request) Response {
	// Step 1: read the displays already saved.
	displays, err := c.listDisplays()
	if err != nil {
		return TechnicalError(err)
	}

	// Step 2: first run (empty table) — detect monitors and insert them.
	if len(displays) == 0 {
		displays, err = c.bootstrapDisplays()
		if err != nil {
			return TechnicalError(err)
		}
	}

	// Step 3: start a wallpaper worker for each display that has a library.
	for _, d := range displays {
		if d.Theme != 0 {
			c.ensureWorker(d)
		}
	}

	// Step 4: convert the list to JSON for the frontend.
	b, err := json.Marshal(displays)
	if err != nil {
		return TechnicalError(err)
	}
	return Response{Type: "browse_displays", Message: string(b)}
}

// editDisplay is the handler for the "edit_display" command. Steps:
//  1. Reject an empty display name.
//  2. Load the previous version of the display (to detect theme changes).
//  3. Apply the patch (theme, timer, video) to the database.
//  4. Only wake the worker when the library (theme) actually changed or the
//     display is new — timer/video edits must not change the wallpaper
//     immediately, because the slider fires an edit on every step while the
//     user is dragging it.
//  5. Return the updated display as JSON.
func (c *Commander) editDisplay(req Request) Response {
	// Step 1: the name identifies the display and is required.
	if req.Display.Name == "" {
		return ErrorResponse("display_empty", "display object is empty")
	}

	// Step 2: read the current values so we can compare the theme later.
	prev, had, err := c.getDisplay(req.Display.Name)
	if err != nil {
		return TechnicalError(err)
	}

	// Step 3: save the editable fields (theme, timer, video).
	updated, err := c.updateDisplay(req.Display.Name, req.Display)
	if err != nil {
		return TechnicalError(err)
	}

	// Step 4: only a library (theme) change wakes the worker right away.
	// The slider fires an edit on every step; waking on each step would
	// change the wallpaper while the user is still dragging.
	if !had || prev.Theme != updated.Theme {
		c.ensureWorker(updated)
		signalWorker(updated.Name)
	}

	// Step 5: convert the updated display to JSON for the frontend.
	b, err := json.Marshal(updated)
	if err != nil {
		return TechnicalError(err)
	}
	return Response{Type: "edit_display", Message: string(b)}
}

// listDisplays reads every display from the database, ordered by id.
func (c *Commander) listDisplays() ([]Display, error) {
	// Run the query with all display columns.
	rows, err := c.database.DB.Query(`SELECT ` + displayColumns + ` FROM displays ORDER BY id`)
	if err != nil {
		return nil, err
	}
	// Close the rows when this function returns.
	defer rows.Close()
	// Scan every row into a Display struct.
	return scanDisplays(rows)
}

// getDisplay loads one display by its monitor name.
// The second return value (bool) says whether a row was found.
func (c *Commander) getDisplay(name string) (Display, bool, error) {
	// Query a single row matching the name.
	row := c.database.DB.QueryRow(`SELECT `+displayColumns+` FROM displays WHERE name = ?`, name)
	d, ok, err := scanDisplay(row)
	if err != nil {
		return Display{}, false, err
	}
	return d, ok, nil
}

// updateDisplay applies the editable fields of a display (theme, timer,
// video) to the database, then re-reads the row and returns the fresh
// values. Only these three fields are written so the update does not
// depend on id/geometry coming from the UI.
func (c *Commander) updateDisplay(name string, patch Display) (Display, error) {
	// Apply only the editable fields — do not rely on ID/geometry from the UI.
	_, err := c.database.DB.Exec(
		`UPDATE displays SET theme = ?, timer = ?, video = ? WHERE name = ?`,
		patch.Theme, patch.Timer, patch.Video, name,
	)
	if err != nil {
		return Display{}, err
	}

	// Read the row back to return the stored values.
	updated, ok, err := c.getDisplay(name)
	if err != nil {
		return Display{}, err
	}
	if !ok {
		// The update matched no row: the display does not exist.
		return Display{}, fmt.Errorf("display not found: %s", name)
	}
	return updated, nil
}

// insertDisplay creates a new row in the displays table with all fields of
// the given display.
func (c *Commander) insertDisplay(d Display) error {
	_, err := c.database.DB.Exec(
		`INSERT INTO displays (name, mirror_of, video, timer, seed, width, height, theme) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Name, d.MirrorOf, d.Video, d.Timer, d.Seed, d.Width, d.Height, d.Theme,
	)
	return err
}

// bootstrapDisplays runs on the first launch, when the displays table is
// empty. Steps:
//  1. Ask hyprctl for the connected monitors.
//  2. For each monitor, set safe defaults (no library, random seed,
//     60 second timer, video off) and insert it.
//  3. Return the full list freshly read from the database.
func (c *Commander) bootstrapDisplays() ([]Display, error) {
	// Step 1: detect the monitors currently connected.
	monitors, err := getMonitors()
	if err != nil {
		return nil, err
	}

	// Step 2: insert each monitor with default settings.
	for _, m := range monitors {
		m.Theme = 0           // no library selected yet
		m.Seed = rand.Int63() // random seed for shuffling wallpapers
		m.Timer = 60          // change wallpaper every 60 seconds
		m.Video = false       // videos disabled by default
		m.ID = 0              // let the database assign the id
		if err := c.insertDisplay(m); err != nil {
			return nil, err
		}
	}

	// Step 3: return everything that is now in the table.
	return c.listDisplays()
}

// scanDisplays reads all rows from an open result set and returns them as
// a slice of Display structs.
func scanDisplays(rows *sql.Rows) ([]Display, error) {
	displays := make([]Display, 0)
	// Advance to the next row until there are no more.
	for rows.Next() {
		d, ok, err := scanDisplay(rows)
		if err != nil {
			return nil, err
		}
		if ok {
			displays = append(displays, d)
		}
	}
	// rows.Err() reports any error that happened during iteration.
	return displays, rows.Err()
}

// rowScanner is the common interface of *sql.Row and *sql.Rows, so both can
// be passed to scanDisplay.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanDisplay reads one row into a Display struct.
// The "video" column is stored as 0/1 in SQLite, so it is converted to a
// bool here. The bool return value says whether a row was found.
func scanDisplay(row rowScanner) (Display, bool, error) {
	var d Display
	var video int
	// Scan all columns into the struct fields.
	err := row.Scan(&d.ID, &d.Name, &d.MirrorOf, &video, &d.Timer, &d.Seed, &d.Width, &d.Height, &d.Theme)
	if err == sql.ErrNoRows {
		// No row matched the query.
		return Display{}, false, nil
	}
	if err != nil {
		return Display{}, false, err
	}
	// Convert the 0/1 integer to a boolean.
	d.Video = video != 0
	return d, true, nil
}

// getMonitors asks hyprctl for the list of connected monitors. Steps:
//  1. Preferred path: run "hyprctl monitors -j" and parse the JSON.
//  2. Fallback path: run "hyprctl monitors" and parse the plain text output
//     (so a demo does not break if JSON parsing fails).
//  3. Return the list of monitors as Display structs.
func getMonitors() ([]Display, error) {
	// Step 1: try the JSON output first (preferred).
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err == nil {
		var mons []Display
		if jerr := json.Unmarshal(out, &mons); jerr == nil && len(mons) > 0 {
			return mons, nil
		}
	}

	// Step 2: fallback — parse simple text (only so the demo keeps working).
	txt, err2 := exec.Command("hyprctl", "monitors").Output()
	if err2 != nil {
		// Both the JSON and the text attempts failed.
		return nil, fmt.Errorf("hyprctl failed (json and text): %v / %v", err, err2)
	}
	// Very simple parse: each line that starts with "Monitor NAME ...".
	var mons []Display
	for _, line := range strings.Split(string(txt), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Monitor ") {
			// Split the line into words; the second word is the monitor name.
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[1]
				mons = append(mons, Display{Name: name})
			}
		}
	}
	if len(mons) == 0 {
		// Step 3: nothing could be parsed from the text.
		return nil, fmt.Errorf("no monitor parsed from text: %s", string(txt))
	}
	return mons, nil
}
