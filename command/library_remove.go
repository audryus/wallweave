package command

import "encoding/json"

func NewHandleLibraryRemove() {
	commands["library_remove"] = handleLibraryRemove
}

func handleLibraryRemove(req Request) Response {
	if req.Path == "" {
		return Response{Type: "error", Message: "Invalid library path"}
	}

	file := openFile(librariesFilePath())
	defer file.Close()

	libraries := readFile[[]Library](file)
	newLibraries := make([]Library, 0, len(libraries))
	for _, lib := range libraries {
		if lib.Path != req.Path {
			newLibraries = append(newLibraries, lib)
		}
	}

	b, err := json.Marshal(newLibraries)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	// grava de volta no arquivo (não em w que no teste é read-only)
	if err := file.Truncate(0); err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	if _, err := file.Seek(0, 0); err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	if _, err := file.Write(b); err != nil {
		return Response{Type: "error", Message: err.Error()}
	}

	return Response{Type: "library_remove"}
}
