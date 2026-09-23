package command

import (
	"encoding/json"
	"testing"
)

func TestGenerateLibrary(t *testing.T) {
	dir := t.TempDir()
	lib, err := generateLibrary(dir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if lib.Path != dir {
		t.Errorf("Expected path %q, got %v", dir, lib.Path)
	}
	if lib.Count.Images != 0 || lib.Count.Videos != 0 {
		t.Errorf("Expected 0/0 em dir vazio, got %v/%v", lib.Count.Images, lib.Count.Videos)
	}
}

func TestAddAndBrowseLibrariesIDs(t *testing.T) {
	c := newTestCommander(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	res := c.Exec(Request{Cmd: "add_library", Path: dir1})
	if res.Type != "add_library" {
		t.Fatalf("Expected add_library, got %v (%v)", res.Type, res.Message)
	}
	var lib1 Library
	if err := json.Unmarshal([]byte(res.Message), &lib1); err != nil {
		t.Fatalf("decode library: %v", err)
	}
	if lib1.ID != 1 {
		t.Errorf("expected first id 1, got %d", lib1.ID)
	}

	res = c.Exec(Request{Cmd: "add_library", Path: dir2})
	if res.Type != "add_library" {
		t.Fatalf("Expected add_library, got %v (%v)", res.Type, res.Message)
	}
	var lib2 Library
	if err := json.Unmarshal([]byte(res.Message), &lib2); err != nil {
		t.Fatalf("decode library: %v", err)
	}
	if lib2.ID != 2 {
		t.Errorf("expected second id 2, got %d", lib2.ID)
	}

	// duplicata
	res = c.Exec(Request{Cmd: "add_library", Path: dir1})
	if res.Type != "error" {
		t.Errorf("Expected duplicate error, got %v (%v)", res.Type, res.Message)
	}

	// browse inclui ids
	res = c.Exec(Request{Cmd: "browse_libraries"})
	if res.Type != "browse_libraries" {
		t.Fatalf("Expected browse_libraries, got %v (%v)", res.Type, res.Message)
	}
	var libs []Library
	if err := json.Unmarshal([]byte(res.Message), &libs); err != nil {
		t.Fatalf("decode libraries: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(libs))
	}
	if libs[0].ID != 1 || libs[1].ID != 2 {
		t.Errorf("expected ids [1 2], got [%d %d]", libs[0].ID, libs[1].ID)
	}

	// remove por id
	res = c.Exec(Request{Cmd: "del_library", ID: 1})
	if res.Type != "del_library" {
		t.Fatalf("Expected del_library, got %v (%v)", res.Type, res.Message)
	}

	res = c.Exec(Request{Cmd: "browse_libraries"})
	if err := json.Unmarshal([]byte(res.Message), &libs); err != nil {
		t.Fatalf("decode libraries: %v", err)
	}
	if len(libs) != 1 || libs[0].ID != 2 {
		t.Errorf("expected remaining id 2, got %+v", libs)
	}

	// remove por path ainda funciona
	res = c.Exec(Request{Cmd: "del_library", Path: dir2})
	if res.Type != "del_library" {
		t.Fatalf("Expected del_library, got %v (%v)", res.Type, res.Message)
	}
}

func TestHandleLibraryInvalidPath(t *testing.T) {
	c := newTestCommander(t)

	res := c.Exec(Request{Cmd: "add_library", Path: ""})
	if res.Type != "error" {
		t.Errorf("Expected error for empty path, got %v", res.Type)
	}

	res = c.Exec(Request{Cmd: "del_library"})
	if res.Type != "error" {
		t.Errorf("Expected error for empty id/path, got %v", res.Type)
	}
}
