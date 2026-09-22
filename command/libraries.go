package command

import (
	"encoding/json"
)

func NewHandleLibraries() {
	commands["libraries"] = handleLibraries
}

func handleLibraries(req Request) Response {
	libraries := getLibraries()

	b, err := json.Marshal(libraries)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	return Response{Type: "libraries", Message: string(b)}
}

func getLibraries() []Library {
	file := openFile(librariesFilePath())
	defer file.Close()

	return readFile[[]Library](file)
}
