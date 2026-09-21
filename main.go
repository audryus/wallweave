// main.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/audryus/wallweave/command"
)

func main() {
	command.RegisterCommands()

	reader := bufio.NewScanner(os.Stdin)
	writer := os.Stdout

	for reader.Scan() {
		line := reader.Bytes()

		var req command.Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(writer, command.Response{Type: "error", Message: "json inválido"})
			continue
		}

		if fn, ok := command.Get(req.Cmd); ok {
			resp := fn(req, writer)
			writeResponse(writer, resp)
			continue
		}
	}
}

func writeResponse(w *os.File, resp command.Response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data) // o \n é o que fecha "a linha" da mensagem
}
