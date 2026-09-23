// main.go
// Entry point of the WallWeave program.
// It opens the database, starts the wallpaper workers, then reads JSON
// commands from stdin (one per line) and writes JSON answers to stdout.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/audryus/wallweave/command"
	"github.com/audryus/wallweave/db"
)

// main is the program entry point. In order, it:
//  1. Opens the SQLite database (creates the file if missing).
//  2. Creates a Commander that can run commands against that database.
//  3. Starts the background workers that rotate wallpapers.
//  4. Loops forever: reads one line of JSON from stdin, runs the command,
//     and writes one line of JSON response to stdout.
func main() {
	// Step 1: open the database file "wallweave.db".
	database, err := db.NewDatabase()
	if err != nil {
		// Could not open the database: print the error and exit with code 1.
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	// Close the database automatically when main returns.
	defer database.Close()

	// Step 2: create the object that knows how to run every command.
	commander := command.NewCommander(database)

	// Step 3: start the wallpaper workers right at boot (no need to wait for
	// the UI to open or for the first timer to fire).
	commander.StartWorkers()

	// Step 4: prepare stdin (line reader) and stdout (writer).
	reader := bufio.NewScanner(os.Stdin)
	writer := os.Stdout

	// Read stdin line by line until the input is closed.
	for reader.Scan() {
		line := reader.Bytes()

		// Parse the line as a Command Request (JSON object).
		var req command.Request
		if err := json.Unmarshal(line, &req); err != nil {
			// The line was not valid JSON: answer with an error and keep going.
			writeResponse(writer, command.Response{Type: "error", Message: "invalid json"})
			continue
		}

		// Run the requested command and send the response back.
		writeResponse(writer, commander.Exec(req))
	}
}

// writeResponse converts a Response to JSON and writes it as one single
// line to the given file (normally stdout). The trailing newline marks the
// end of the message so the frontend knows the response is complete.
func writeResponse(w *os.File, resp command.Response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", data)
}
