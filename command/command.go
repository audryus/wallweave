package command

import (
	"encoding/json"
	"os"
)

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

type CommandFunc func(Request) Response

var commands = make(map[string]CommandFunc)

func RegisterCommands() {
	NewHandleStatus()
	NewHandleLibraries()
	NewHandleLibrary()
	NewHandleLibraryRemove()
	NewHandleDisplay()
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

func readFile[V any](file *os.File) V {
	var result V
	// garante leitura do início — readFile pode ser chamado com offset no EOF
	if _, err := file.Seek(0, 0); err != nil {
		return result
	}
	// arquivo vazio (0 bytes) -> EOF, retorna zero value (slice nil -> append funciona)
	if info, err := file.Stat(); err == nil && info.Size() == 0 {
		return result
	}
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		// EOF ou JSON inválido em arquivo recém-criado -> trata como vazio
		return result
	}
	return result
}

func openFile(name string) *os.File {
	// O_RDWR precisa para ler e depois truncar/escrever; O_CREATE cria se não existir
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	return file
}
