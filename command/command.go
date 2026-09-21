package command

import "os"

// Request representa qualquer comando que chega do frontend.
type Request struct {
	Cmd  string `json:"cmd"`
	Path string `json:"path,omitempty"`
}

// Response é o envelope genérico de resposta.
type Response struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type CommandFunc func(Request, *os.File) Response

var commands = make(map[string]CommandFunc)

func RegisterCommands() {
	NewHandleStatus()
	NewHandleLibraries()
	NewHandleLibrary()
}

func Get(cmd string) (CommandFunc, bool) {
	fn, ok := commands[cmd]
	return fn, ok
}

type LibraryCount struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
}

type Library struct {
	Path   string       `json:"path"`
	Thumbs []string     `json:"thumbs"`
	Count  LibraryCount `json:"count"`
}
