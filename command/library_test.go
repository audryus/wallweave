package command

import (
	"encoding/json"
	"testing"
)

// TestGenerateLibrary checks that generateLibrary works on an empty
// folder: it must not fail, it must keep the given path, and it must
// report 0 images and 0 videos.
func TestGenerateLibrary(t *testing.T) {
	// Create an empty temporary folder for this test.
	dir := t.TempDir()
	lib, err := generateLibrary(dir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// The library must keep the folder path we passed in.
	if lib.Path != dir {
		t.Errorf("Expected path %q, got %v", dir, lib.Path)
	}
	// An empty folder has no images and no videos.
	if lib.Count.Images != 0 || lib.Count.Videos != 0 {
		t.Errorf("Expected 0/0 in empty dir, got %v/%v", lib.Count.Images, lib.Count.Videos)
	}
}

// TestAddAndBrowseLibrariesIDs walks through the whole library lifecycle:
// add two libraries (ids 1 and 2), reject a duplicate, list them (with
// their ids), delete by id, and finally delete by path.
func TestAddAndBrowseLibrariesIDs(t *testing.T) {
	// Create a commander backed by a temporary database.
	c := newTestCommander(t)
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Add the first library — it should get id 1.
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

	// Add the second library — it should get id 2.
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

	// Adding the same folder again must fail with a stable error code.
	res = c.Exec(Request{Cmd: "add_library", Path: dir1})
	if res.Type != "error" {
		t.Errorf("Expected duplicate error, got %v (%v)", res.Type, res.Message)
	}
	if res.Code != "folder_exists" {
		t.Errorf("Expected code folder_exists, got %q", res.Code)
	}

	// Browse must return both libraries, including their ids.
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

	// Delete by id: remove the first library (id 1).
	res = c.Exec(Request{Cmd: "del_library", ID: 1})
	if res.Type != "del_library" {
		t.Fatalf("Expected del_library, got %v (%v)", res.Type, res.Message)
	}

	// After deleting id 1, only the second library (id 2) must remain.
	res = c.Exec(Request{Cmd: "browse_libraries"})
	if err := json.Unmarshal([]byte(res.Message), &libs); err != nil {
		t.Fatalf("decode libraries: %v", err)
	}
	if len(libs) != 1 || libs[0].ID != 2 {
		t.Errorf("expected remaining id 2, got %+v", libs)
	}

	// Delete by path still works for the remaining library.
	res = c.Exec(Request{Cmd: "del_library", Path: dir2})
	if res.Type != "del_library" {
		t.Fatalf("Expected del_library, got %v (%v)", res.Type, res.Message)
	}
}

// TestHandleLibraryInvalidPath checks that invalid requests are rejected:
// an empty path on add, and a delete without id or path.
func TestHandleLibraryInvalidPath(t *testing.T) {
	// Create a commander backed by a temporary database.
	c := newTestCommander(t)

	// Adding a library with an empty path must fail with a stable code.
	res := c.Exec(Request{Cmd: "add_library", Path: ""})
	if res.Type != "error" {
		t.Errorf("Expected error for empty path, got %v", res.Type)
	}
	if res.Code != "invalid_path" {
		t.Errorf("Expected code invalid_path, got %q", res.Code)
	}

	// Deleting a library without id or path must fail with a stable code.
	res = c.Exec(Request{Cmd: "del_library"})
	if res.Type != "error" {
		t.Errorf("Expected error for empty id/path, got %v", res.Type)
	}
	if res.Code != "invalid_id_path" {
		t.Errorf("Expected code invalid_id_path, got %q", res.Code)
	}
}
