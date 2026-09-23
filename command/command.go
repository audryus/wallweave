package command

import "github.com/audryus/wallweave/db"

// Commander registra e executa comandos com acesso ao banco.
type Commander struct {
	database db.Database
	commands map[string]CommandFunc
}

func NewCommander(database db.Database) *Commander {
	c := &Commander{
		database: database,
		commands: make(map[string]CommandFunc),
	}
	c.registerStatus()
	c.registerLibrary()
	c.registerDisplay()
	return c
}

func (c *Commander) Register(name string, fn CommandFunc) {
	c.commands[name] = fn
}

func (c *Commander) Exec(req Request) Response {
	fn, ok := c.commands[req.Cmd]
	if !ok {
		return Response{Type: "error", Message: "unknown command: " + req.Cmd}
	}
	return fn(req)
}

// Request representa qualquer comando que chega do frontend.
type Request struct {
	Cmd     string  `json:"cmd"`
	ID      int     `json:"id,omitempty"`
	Path    string  `json:"path,omitempty"`
	Display Display `json:"display,omitempty"`
}

// Response é o envelope genérico de resposta.
type Response struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type CommandFunc func(Request) Response

type LibraryCount struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
}

type Library struct {
	ID     int          `json:"id"`
	Path   string       `json:"path"`
	Thumbs []string     `json:"thumbs"`
	Count  LibraryCount `json:"count"`
}
