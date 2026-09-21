package command

import (
	"encoding/json"
	"os"
)

func NewHandleLibraries() {
	commands["libraries"] = func(req Request, w *os.File) Response {
		file := openFile(librariesFilePath())
		defer file.Close()

		libraries := readFile[[]Library](file)

		b, err := json.Marshal(libraries)
		if err != nil {
			return Response{Type: "error", Message: err.Error()}
		}
		return Response{Type: "libraries", Message: string(b)}
	}
}
