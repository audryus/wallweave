package command

import (
	"encoding/json"
	"os"
	"os/exec"
)

func NewHandleStatus() {
	type Status struct {
		Mpvpaper  bool   `json:"mpvpaper"`
		Hyprpaper bool   `json:"hyprpaper"`
		Video     string `json:"video"`
		Image     string `json:"image"`
		Color     string `json:"color"`
		Label     string `json:"label"`
		Connected bool   `json:"connected"`
	}

	commands["status"] = func(req Request, w *os.File) Response {

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
			status.Hyprpaper = false
			status.Color = "#f1c40f"
			status.Image = "Will use Omarchy (global, not per-monitor)"
			status.Label = "Degraded"
		}

		b, err := json.Marshal(status)
		if err != nil {
			return Response{Type: "error", Message: "erro ao serializar status"}
		}
		return Response{Type: "status", Message: string(b)}
	}
}
