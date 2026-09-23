// main.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/audryus/wallweave/command"
	"github.com/audryus/wallweave/db"
)

func main() {
	database, err := db.NewDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	commander := command.NewCommander(database)
	// workers sobem já no boot (dispensando a UI / o primeiro timer)
	commander.StartWorkers()

	reader := bufio.NewScanner(os.Stdin)
	writer := os.Stdout

	for reader.Scan() {
		line := reader.Bytes()

		var req command.Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(writer, command.Response{Type: "error", Message: "json inválido"})
			continue
		}

		writeResponse(writer, commander.Exec(req))
	}
}

func writeResponse(w *os.File, resp command.Response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data) // o \n é o que fecha "a linha" da mensagem
}
