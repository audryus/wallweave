package command

import (
	"database/sql"
	"encoding/json"
	"os/exec"
)

// Status holds the health information of the wallpaper tools:
// whether mpvpaper and hyprpaper are installed, plus a color and stable
// message codes the UI translates, and whether the backend is connected.
// Label, Video, and Image are i18n keys (e.g. "running", "mpvpaper_missing",
// "omarchy_fallback") — never English sentences.
type Status struct {
	Mpvpaper  bool   `json:"mpvpaper"`
	Hyprpaper bool   `json:"hyprpaper"`
	Video     string `json:"video"`
	Image     string `json:"image"`
	Color     string `json:"color"`
	Label     string `json:"label"`
	Connected bool   `json:"connected"`
}

// registerStatus registers the "get_status" command so the frontend can
// ask for the current health of the system.
func (c *Commander) registerStatus() {
	c.Register("get_status", c.getStatus)
}

// getStatus is the handler for the "get_status" command. It:
//  1. Probes the system (checks if mpvpaper and hyprpaper exist).
//  2. Saves the result in the database (so it can be loaded later).
//  3. Serializes the status to JSON and returns it to the frontend.
func (c *Commander) getStatus(req Request) Response {
	// Step 1: check which wallpaper tools are installed.
	status := probeStatus()

	// Step 2: persist the status in the database.
	if err := c.saveStatus(status); err != nil {
		return Response{Type: "error", Message: err.Error()}
	}

	// Step 3: convert the status to JSON for the frontend.
	b, err := json.Marshal(status)
	if err != nil {
		return ErrorResponse("serialize_status", "error serializing status")
	}

	// Answer with the status payload.
	return Response{Type: "get_status", Message: string(b)}
}

// probeStatus checks the machine for the required tools:
//   - If mpvpaper is missing, the status becomes "degraded" (yellow).
//   - If hyprpaper is missing, the status becomes "degraded" too.
//   - When both exist, the status is "running" (green).
//
// Label/Video/Image hold stable codes the UI translates — never English
// sentences. It never fails; it only returns what it found.
func probeStatus() Status {
	var status Status
	// Assume the best case first: everything installed and running.
	status.Color = "#2ecc71"
	status.Label = "running"
	status.Connected = true

	// Check if the "mpvpaper" executable exists in PATH.
	if _, err := exec.LookPath("mpvpaper"); err == nil {
		status.Mpvpaper = true
	} else {
		// mpvpaper is missing: mark as degraded and set the reason code.
		status.Color = "#f1c40f"
		status.Video = "mpvpaper_missing"
		status.Label = "degraded"
	}
	// Check if the "hyprpaper" executable exists in PATH.
	if _, err := exec.LookPath("hyprpaper"); err == nil {
		status.Hyprpaper = true
	} else {
		// hyprpaper is missing: mark as degraded and set the fallback code.
		status.Color = "#f1c40f"
		status.Image = "omarchy_fallback"
		status.Label = "degraded"
	}
	return status
}

// saveStatus writes (or overwrites) the status row in the database.
// There is only one status row (id = 1), so it uses an "upsert":
// insert if it does not exist, update all fields if it does.
func (c *Commander) saveStatus(s Status) error {
	_, err := c.database.DB.Exec(
		`INSERT INTO status (id, mpvpaper, hyprpaper, video, image, color, label, connected, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
			mpvpaper = excluded.mpvpaper,
			hyprpaper = excluded.hyprpaper,
			video = excluded.video,
			image = excluded.image,
			color = excluded.color,
			label = excluded.label,
			connected = excluded.connected,
			updated_at = excluded.updated_at`,
		s.Mpvpaper, s.Hyprpaper, s.Video, s.Image, s.Color, s.Label, s.Connected,
	)
	return err
}

// loadStatus reads the saved status from the database.
// If no row exists yet it returns an empty Status and no error.
// Booleans are stored as 0/1 integers, so they are converted back here.
func (c *Commander) loadStatus() (Status, error) {
	var s Status
	var mpvpaper, hyprpaper, connected int
	// Query the single status row (id = 1).
	err := c.database.DB.QueryRow(
		`SELECT mpvpaper, hyprpaper, video, image, color, label, connected FROM status WHERE id = 1`,
	).Scan(&mpvpaper, &hyprpaper, &s.Video, &s.Image, &s.Color, &s.Label, &connected)
	if err == sql.ErrNoRows {
		// Nothing saved yet: return an empty status.
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	// Convert the 0/1 integers back to booleans.
	s.Mpvpaper = mpvpaper != 0
	s.Hyprpaper = hyprpaper != 0
	s.Connected = connected != 0
	return s, nil
}
