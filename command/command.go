// Package command implements the command layer of WallWeave: it receives
// JSON requests from the frontend, runs the right handler, and returns a
// JSON response. Handlers have access to the SQLite database.
package command

import "github.com/audryus/wallweave/db"

// Commander registers and runs commands, and keeps a reference to the
// database that every handler can use.
type Commander struct {
	database db.Database
	commands map[string]CommandFunc
}

// NewCommander creates a Commander for the given database and registers
// all built-in commands (status, libraries, displays).
func NewCommander(database db.Database) *Commander {
	c := &Commander{
		database: database,
		commands: make(map[string]CommandFunc),
	}
	// Register each command group.
	c.registerStatus()
	c.registerLibrary()
	c.registerDisplay()
	return c
}

// Register maps a command name (for example "add_library") to the function
// that will run when that command arrives.
func (c *Commander) Register(name string, fn CommandFunc) {
	c.commands[name] = fn
}

// Exec looks up the command name in the request and runs its handler.
// If the command name is unknown it returns an error response with a
// stable code the UI can translate (technical message stays for logs).
func (c *Commander) Exec(req Request) Response {
	fn, ok := c.commands[req.Cmd]
	if !ok {
		return ErrorResponse("unknown_command", "unknown command: "+req.Cmd)
	}
	return fn(req)
}

// Request represents any command that arrives from the frontend.
// Only the fields relevant to the command are filled in.
type Request struct {
	Cmd     string  `json:"cmd"`
	ID      int     `json:"id,omitempty"`
	Path    string  `json:"path,omitempty"`
	Display Display `json:"display,omitempty"`
}

// Response is the generic answer envelope sent back to the frontend.
// Type says which command answered; Message carries the payload (often JSON)
// or the technical error text. Code is a stable, translatable error key
// (e.g. "folder_exists") used only when Type is "error"; the UI maps it
// through I18n and falls back to Message for unknown/technical errors.
type Response struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrorResponse builds a Type:"error" response with a stable code for the
// UI and a technical message for logs/console.
func ErrorResponse(code, message string) Response {
	return Response{Type: "error", Code: code, Message: message}
}

// TechnicalError builds a Type:"error" response from a raw Go error.
// No code: the message is developer-facing and is not translated.
func TechnicalError(err error) Response {
	return Response{Type: "error", Message: err.Error()}
}

// CommandFunc is the signature every command handler must follow: it takes
// a Request and returns a Response.
type CommandFunc func(Request) Response

// LibraryCount holds how many images and videos a library contains.
type LibraryCount struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
}

// Library represents one wallpaper folder that the user added.
// Thumbs holds the paths of the generated thumbnail images.
type Library struct {
	ID     int          `json:"id"`
	Path   string       `json:"path"`
	Thumbs []string     `json:"thumbs"`
	Count  LibraryCount `json:"count"`
}
