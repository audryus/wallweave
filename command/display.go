package command

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
)

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

const displayColumns = `id, name, mirror_of, video, timer, seed, width, height, theme`

func (c *Commander) registerDisplay() {
	c.Register("browse_displays", c.browseDisplays)
	c.Register("edit_display", c.editDisplay)
}

func (c *Commander) browseDisplays(req Request) Response {
	displays, err := c.listDisplays()
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}

	if len(displays) == 0 {
		displays, err = c.bootstrapDisplays()
		if err != nil {
			return Response{Type: "error", Message: err.Error()}
		}
	}

	for _, d := range displays {
		if d.Theme != 0 {
			c.ensureWorker(d)
		}
	}

	b, err := json.Marshal(displays)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	return Response{Type: "browse_displays", Message: string(b)}
}

func (c *Commander) editDisplay(req Request) Response {
	if req.Display.Name == "" {
		return Response{Type: "error", Message: "display object is empty"}
	}

	prev, had, err := c.getDisplay(req.Display.Name)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}

	updated, err := c.updateDisplay(req.Display.Name, req.Display)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}

	// só troca de library acorda o worker na hora — timer/video não
	// (slider dispara edit a cada passo; acordar trocaria wallpaper no arraste)
	if !had || prev.Theme != updated.Theme {
		c.ensureWorker(updated)
		signalWorker(updated.Name)
	}

	b, err := json.Marshal(updated)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	return Response{Type: "edit_display", Message: string(b)}
}

func (c *Commander) listDisplays() ([]Display, error) {
	rows, err := c.database.DB.Query(`SELECT ` + displayColumns + ` FROM displays ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDisplays(rows)
}

func (c *Commander) getDisplay(name string) (Display, bool, error) {
	row := c.database.DB.QueryRow(`SELECT `+displayColumns+` FROM displays WHERE name = ?`, name)
	d, ok, err := scanDisplay(row)
	if err != nil {
		return Display{}, false, err
	}
	return d, ok, nil
}

func (c *Commander) updateDisplay(name string, patch Display) (Display, error) {
	// aplica só os campos editáveis — não depende de ID/geometry vindos da UI
	_, err := c.database.DB.Exec(
		`UPDATE displays SET theme = ?, timer = ?, video = ? WHERE name = ?`,
		patch.Theme, patch.Timer, patch.Video, name,
	)
	if err != nil {
		return Display{}, err
	}

	updated, ok, err := c.getDisplay(name)
	if err != nil {
		return Display{}, err
	}
	if !ok {
		return Display{}, fmt.Errorf("display not found: %s", name)
	}
	return updated, nil
}

func (c *Commander) insertDisplay(d Display) error {
	_, err := c.database.DB.Exec(
		`INSERT INTO displays (name, mirror_of, video, timer, seed, width, height, theme) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Name, d.MirrorOf, d.Video, d.Timer, d.Seed, d.Width, d.Height, d.Theme,
	)
	return err
}

func (c *Commander) bootstrapDisplays() ([]Display, error) {
	monitors, err := getMonitors()
	if err != nil {
		return nil, err
	}

	for _, m := range monitors {
		m.Theme = 0
		m.Seed = rand.Int63()
		m.Timer = 60
		m.Video = false
		m.ID = 0
		if err := c.insertDisplay(m); err != nil {
			return nil, err
		}
	}

	return c.listDisplays()
}

func scanDisplays(rows *sql.Rows) ([]Display, error) {
	displays := make([]Display, 0)
	for rows.Next() {
		d, ok, err := scanDisplay(rows)
		if err != nil {
			return nil, err
		}
		if ok {
			displays = append(displays, d)
		}
	}
	return displays, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDisplay(row rowScanner) (Display, bool, error) {
	var d Display
	var video int
	err := row.Scan(&d.ID, &d.Name, &d.MirrorOf, &video, &d.Timer, &d.Seed, &d.Width, &d.Height, &d.Theme)
	if err == sql.ErrNoRows {
		return Display{}, false, nil
	}
	if err != nil {
		return Display{}, false, err
	}
	d.Video = video != 0
	return d, true, nil
}

func getMonitors() ([]Display, error) {
	// tentativa JSON - preferida
	out, err := exec.Command("hyprctl", "monitors", "-j").Output()
	if err == nil {
		var mons []Display
		if jerr := json.Unmarshal(out, &mons); jerr == nil && len(mons) > 0 {
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
