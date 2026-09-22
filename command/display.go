package command

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
)

type DisplayTheme struct {
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
}

type Display struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	MirrorOf string         `json:"mirrorOf"`
	Video    bool           `json:"video"`
	Timer    int            `json:"timer"`
	Seed     int64          `json:"seed"`
	Themes   []DisplayTheme `json:"themes"`
}

func NewHandleDisplay() {
	commands["display"] = handleDisplay
}

func handleDisplay(req Request) Response {
	displays, err := getMonitors()
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	libraries := getLibraries()
	themes := make([]DisplayTheme, 0)

	themes = append(themes, DisplayTheme{
		Name:     "Omarchy global (not per-display)",
		Selected: true,
	})

	for i := range libraries {
		lib := libraries[i]
		themes = append(themes, DisplayTheme{
			Name:     lib.Path,
			Selected: false,
		})
	}

	for i := range displays {
		displays[i].Themes = append(displays[i].Themes, themes...)
		displays[i].Seed = rand.Int63()
	}

	b, err := json.Marshal(displays)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	return Response{Type: "display", Message: string(b)}
}

func getMonitors() ([]Display, error) {
	// tentativa JSON - preferida
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err == nil {
		var mons []Display
		if jerr := json.Unmarshal(out, &mons); jerr == nil {
			return mons, nil
		}
	}

	// fallback: parse texto simples (só pra não quebrar demo)
	txt, err2 := exec.Command("hyprctl", "monitors").Output()
	if err2 != nil {
		return nil, fmt.Errorf("hyprctl falhou (json e texto): %v / %v", err, err2)
	}
	// parse bem simples: cada linha "Monitor NAME ..."
	var mons []Display
	for _, line := range strings.Split(string(txt), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Monitor ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				name := parts[1]
				mons = append(mons, Display{Name: name})
			}
		}
	}
	if len(mons) == 0 {
		return nil, fmt.Errorf("nenhum monitor parseado do texto: %s", string(txt))
	}
	return mons, nil
}
