package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func librariesFilePath() string {
	// segue bkp/Wallweave.qml: pluginDir + "/libraries.json"
	// respeita XDG_CONFIG_HOME, fallback para ~/.config
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	// plugin instalado via symlink ~/.config/omarchy/plugins/audryus.wallweave
	pluginDir := filepath.Join(base, "omarchy", "plugins", "audryus.wallweave")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		// fallback para cwd se não conseguir criar
		return "libraries.json"
	}
	return filepath.Join(pluginDir, "libraries.json")
}

func NewHandleLibrary() {
	commands["library"] = handleLibrary
}

func handleLibrary(req Request) Response {
	if req.Path == "" {
		return Response{Type: "error", Message: "Invalid library path"}
	}

	file := openFile(librariesFilePath())
	defer file.Close()

	libraries := readFile[[]Library](file)
	for i := range libraries {
		lib := libraries[i]
		if lib.Path == req.Path {
			return Response{Type: "error", Message: "Folder already added"}
		}
	}

	// valida, conta e gera thumbs — corrige generateVideoThumb/processo
	library, err := generateLibrary(req.Path)
	if err != nil {
		return Response{Type: "error", Message: err.Error()}
	}
	libraries = append(libraries, library)

	b, err := json.Marshal(libraries)
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

	return Response{Type: "library"}
}

// generate: validate the path exists and is a directory, count images and videos, generate thumbs (if possible), return Library struct with thumbs and counts.
func generateLibrary(path string) (Library, error) {
	var library Library
	library.Path = path

	info, err := os.Stat(path)
	if err != nil {
		return library, err
	}
	if !info.IsDir() {
		return library, os.ErrInvalid
	}

	// create a thumbs directory inside the library path if it doesn't exist
	thumbsDir := filepath.Join(path, "thumbs")
	if _, err := os.Stat(thumbsDir); os.IsNotExist(err) {
		if err := os.Mkdir(thumbsDir, 0755); err != nil {
			return library, err
		}
	}

	// Count images and videos — ignora thumbsDir para não contar thumbs gerados
	var imagesCount, videosCount int
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// não entra no thumbsDir
		if info.IsDir() && p == thumbsDir {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			// evita thumbs recém-gerados se Walk já entrou antes do Skip
			if filepath.Dir(p) == thumbsDir {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(info.Name()))
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp":
				imagesCount++
				// ignora erro de thumb para não abortar Walk completo
				_ = generateThumb(p, thumbsDir)
			case ".mp4", ".avi", ".mov", ".mkv", ".webm":
				videosCount++
				_ = generateVideoThumb(p, thumbsDir)
			}
		}
		return nil
	})
	if err != nil {
		return library, err
	}

	library.Count.Images = imagesCount
	library.Count.Videos = videosCount

	// Coleta thumbs reais gerados em thumbsDir
	if entries, err := os.ReadDir(thumbsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jpg") {
				library.Thumbs = append(library.Thumbs, filepath.Join(thumbsDir, e.Name()))
			}
		}
	}

	return library, nil
}

func generateVideoThumb(filePath string, thumbsDir string) error {
	// nome sem dupla extensão: video.mp4 -> video.jpg (não video.mp4.jpg)
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	savePath := filepath.Join(thumbsDir, base+".jpg")

	// -y sobrescreve, -i filePath direto (seekable, evita pipe: que quebra em mp4 com moov no fim)
	// thumbnail escolhe frame representativo, scale limita a 320px
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-vframes", "1", "-vf", "thumbnail,scale=320:240:flags=lanczos", "-q:v", "2", savePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg %s: %w: %s", filepath.Base(filePath), err, stderr.String())
	}
	return nil
}

func generateThumb(filePath string, thumbsDir string) error {
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	savePath := filepath.Join(thumbsDir, base+".jpg")
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		return nil
	}
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-vframes", "1", "-vf", "scale=320:240:flags=lanczos", "-q:v", "2", savePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumb %s: %w: %s", filepath.Base(filePath), err, stderr.String())
	}
	return nil
}
