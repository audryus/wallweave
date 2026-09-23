package command

import (
	"database/sql"
	"encoding/json"
	"os/exec"
)

type Status struct {
	Mpvpaper  bool   `json:"mpvpaper"`
	Hyprpaper bool   `json:"hyprpaper"`
	Video     string `json:"video"`
	Image     string `json:"image"`
	Color     string `json:"color"`
	Label     string `json:"label"`
	Connected bool   `json:"connected"`
}

func (c *Commander) registerStatus() {
	c.Register("get_status", c.getStatus)
}

func (c *Commander) getStatus(req Request) Response {
	status := probeStatus()

	if err := c.saveStatus(status); err != nil {
		return Response{Type: "error", Message: err.Error()}
	}

	b, err := json.Marshal(status)
	if err != nil {
		return Response{Type: "error", Message: "erro ao serializar status"}
	}

	return Response{Type: "get_status", Message: string(b)}
}

func probeStatus() Status {
	var status Status
	status.Color = "#2ecc71"
	status.Label = "Running"
	status.Connected = true

	if _, err := exec.LookPath("mpvpaper"); err == nil {
		status.Mpvpaper = true
	} else {
		status.Color = "#f1c40f"
		status.Video = "mpvpaper not installed"
		status.Label = "Degraded"
	}
	if _, err := exec.LookPath("hyprpaper"); err == nil {
		status.Hyprpaper = true
	} else {
		status.Color = "#f1c40f"
		status.Image = "Will use Omarchy (global, not per-monitor)"
		status.Label = "Degraded"
	}
	return status
}

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

func (c *Commander) loadStatus() (Status, error) {
	var s Status
	var mpvpaper, hyprpaper, connected int
	err := c.database.DB.QueryRow(
		`SELECT mpvpaper, hyprpaper, video, image, color, label, connected FROM status WHERE id = 1`,
	).Scan(&mpvpaper, &hyprpaper, &s.Video, &s.Image, &s.Color, &s.Label, &connected)
	if err == sql.ErrNoRows {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	s.Mpvpaper = mpvpaper != 0
	s.Hyprpaper = hyprpaper != 0
	s.Connected = connected != 0
	return s, nil
}
