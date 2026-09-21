package command

import (
	"testing"
)

func TestOpenFile(t *testing.T) {
	openFile("teste.json")
}

func TestHandleLibrary(t *testing.T) {
	// usa diretório temporário válido — generateLibrary valida IsDir
	dir := t.TempDir()
	file := openFile("libraries.json")
	defer file.Close()

	response := handleLibrary(Request{
		Path: dir,
	}, file)
	if response.Type != "library" {
		t.Errorf("Expected library response, got %v (%v)", response.Type, response.Message)
	}
}

func TestGenerateLibrary(t *testing.T) {
	dir := t.TempDir()
	// cria um jpg e um mp4 fake para contar
	// ffmpeg já cria sample válido
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
