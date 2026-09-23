package command

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// registerLibrary registers the commands used to manage wallpaper folders:
// add a library, delete a library, and list all libraries.
func (c *Commander) registerLibrary() {
	c.Register("add_library", c.addLibrary)
	c.Register("del_library", c.delLibrary)
	c.Register("browse_libraries", c.browseLibraries)
}

// addLibrary is the handler for the "add_library" command. Steps:
//  1. Reject an empty path.
//  2. Check the folder was not already added.
//  3. Scan the folder: count images/videos and generate thumbnails.
//  4. Insert the new library row in the database.
//  5. Return the created library as JSON.
func (c *Commander) addLibrary(req Request) Response {
	// Step 1: the path is required.
	if req.Path == "" {
		return ErrorResponse("invalid_path", "library path is empty")
	}

	// Step 2: reject a folder that is already in the list.
	exists, err := c.libraryExists(req.Path)
	if err != nil {
		return TechnicalError(err)
	}
	if exists {
		return ErrorResponse("folder_exists", "folder already in library")
	}

	// Step 3: walk the folder, count files, and create thumbnails.
	library, err := generateLibrary(req.Path)
	if err != nil {
		return TechnicalError(err)
	}

	// Step 4: save the library in the database.
	if err := c.insertLibrary(&library); err != nil {
		return TechnicalError(err)
	}

	// Step 5: convert the library to JSON for the frontend.
	b, err := json.Marshal(library)
	if err != nil {
		return TechnicalError(err)
	}
	return Response{Type: "add_library", Message: string(b)}
}

// generateLibrary scans a folder and builds a Library struct. Steps:
//  1. Check the path exists and is a directory.
//  2. Create a "thumbs" folder inside it if it does not exist.
//  3. Walk every file: count images and videos (skipping the thumbs folder)
//     and generate a thumbnail for each media file with ffmpeg.
//  4. Store the image/video counts.
//  5. Collect the paths of all generated thumbnail files.
func generateLibrary(path string) (Library, error) {
	var library Library
	library.Path = path

	// Step 1: the path must exist and must be a directory.
	info, err := os.Stat(path)
	if err != nil {
		return library, err
	}
	if !info.IsDir() {
		return library, os.ErrInvalid
	}

	// Step 2: create a thumbs directory inside the library path if it does
	// not exist.
	thumbsDir := filepath.Join(path, "thumbs")
	if _, err := os.Stat(thumbsDir); os.IsNotExist(err) {
		if err := os.Mkdir(thumbsDir, 0755); err != nil {
			return library, err
		}
	}

	// Step 3: walk the whole folder, counting images and videos. The thumbs
	// folder itself is skipped so generated thumbnails are not counted.
	var imagesCount, videosCount int
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Do not enter the thumbs directory.
		if info.IsDir() && p == thumbsDir {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			// Safety: skip any file that lives inside thumbsDir, in case the
			// walk already entered it before SkipDir could take effect.
			if filepath.Dir(p) == thumbsDir {
				return nil
			}
			// Count the file by its extension and generate its thumbnail.
			ext := strings.ToLower(filepath.Ext(info.Name()))
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp":
				imagesCount++
				// Ignore thumbnail errors so one bad file does not abort
				// the whole walk.
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

	// Step 4: store the counts in the library struct.
	library.Count.Images = imagesCount
	library.Count.Videos = videosCount

	// Step 5: collect the paths of the thumbnails that were actually
	// generated (only .jpg files inside the thumbs folder).
	if entries, err := os.ReadDir(thumbsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jpg") {
				library.Thumbs = append(library.Thumbs, filepath.Join(thumbsDir, e.Name()))
			}
		}
	}

	return library, nil
}

// generateVideoThumb creates a thumbnail image for a video file using
// ffmpeg. Steps:
//  1. Build the output name without a double extension
//     (video.mp4 -> video.jpg, not video.mp4.jpg).
//  2. Run ffmpeg: overwrite the output, read the input directly (seekable,
//     avoids the broken pipe: approach for mp4 files with moov at the end),
//     pick a representative frame, and scale it down to 320x240.
func generateVideoThumb(filePath string, thumbsDir string) error {
	// Step 1: strip the original extension to build the thumbnail name.
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	savePath := filepath.Join(thumbsDir, base+".jpg")

	// Step 2: run ffmpeg. -y overwrites, -i reads the file directly,
	// "thumbnail" picks a representative frame, "scale" limits it to 320px.
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-vframes", "1", "-vf", "thumbnail,scale=320:240:flags=lanczos", "-q:v", "2", savePath)
	// Capture stderr so the error message can include ffmpeg's own output.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg %s: %w: %s", filepath.Base(filePath), err, stderr.String())
	}
	return nil
}

// generateThumb creates a thumbnail image for an image file using ffmpeg.
// Steps:
//  1. Build the output name without a double extension.
//  2. Only continue for known image extensions; ignore anything else.
//  3. Run ffmpeg to decode the first frame and scale it to 320x240.
func generateThumb(filePath string, thumbsDir string) error {
	// Step 1: strip the original extension to build the thumbnail name.
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	savePath := filepath.Join(thumbsDir, base+".jpg")
	// Step 2: only handle known image types.
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		return nil
	}
	// Step 3: run ffmpeg to produce the scaled thumbnail.
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-vframes", "1", "-vf", "scale=320:240:flags=lanczos", "-q:v", "2", savePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumb %s: %w: %s", filepath.Base(filePath), err, stderr.String())
	}
	return nil
}

// delLibrary is the handler for the "del_library" command. Steps:
//  1. Require an id or a path (at least one of them).
//  2. Delete the row from the database (by id if given, otherwise by path).
//  3. Wake all wallpaper workers: a display that pointed to this library
//     will fall back to its defensive path (restore default wallpaper).
func (c *Commander) delLibrary(req Request) Response {
	// Step 1: both fields empty means the request is invalid.
	if req.ID == 0 && req.Path == "" {
		return ErrorResponse("invalid_id_path", "library id/path missing")
	}

	// Step 2: delete by id when an id was sent, otherwise by path.
	var err error
	if req.ID != 0 {
		err = c.deleteLibraryByID(req.ID)
	} else {
		err = c.deleteLibrary(req.Path)
	}
	if err != nil {
		return TechnicalError(err)
	}

	// Step 3: wake the workers so they notice the library is gone.
	signalAllWorkers()

	return Response{Type: "del_library"}
}

// browseLibraries is the handler for the "browse_libraries" command. It
// loads every library from the database and returns them as a JSON array.
func (c *Commander) browseLibraries(req Request) Response {
	// Load all libraries ordered by id.
	libraries, err := c.fetchLibraries()
	if err != nil {
		return TechnicalError(err)
	}

	// Convert the list to JSON for the frontend.
	b, err := json.Marshal(libraries)
	if err != nil {
		return TechnicalError(err)
	}
	return Response{Type: "browse_libraries", Message: string(b)}
}

// fetchLibraries reads every library row from the database, ordered by id.
// The thumbs column stores a JSON string, so it is decoded into the
// Thumbs slice of each library.
func (c *Commander) fetchLibraries() ([]Library, error) {
	libraries := make([]Library, 0)

	// Run the query and close the rows when done.
	rows, err := c.database.DB.Query(`SELECT id, path, thumbs, images, videos FROM libraries ORDER BY id`)
	if err != nil {
		return libraries, err
	}
	defer rows.Close()

	// Read one row at a time.
	for rows.Next() {
		var lib Library
		var thumbs string
		// Scan the columns; thumbs is still a JSON string here.
		if err := rows.Scan(&lib.ID, &lib.Path, &thumbs, &lib.Count.Images, &lib.Count.Videos); err != nil {
			return nil, err
		}
		// Decode the thumbs JSON string into the slice.
		if thumbs != "" {
			if err := json.Unmarshal([]byte(thumbs), &lib.Thumbs); err != nil {
				return nil, err
			}
		}
		libraries = append(libraries, lib)
	}
	// Return the list plus any error that happened during iteration.
	return libraries, rows.Err()
}

// libraryExists reports whether a library with the given path is already in
// the database. It returns (false, nil) when no row matches.
func (c *Commander) libraryExists(path string) (bool, error) {
	var one int
	// SELECT 1 returns one row only if the path exists.
	err := c.database.DB.QueryRow(`SELECT 1 FROM libraries WHERE path = ?`, path).Scan(&one)
	if err == sql.ErrNoRows {
		// No row found: the path does not exist yet.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// A row was found: the path already exists.
	return true, nil
}

// insertLibrary saves a new library row in the database and fills in the
// library ID (from the auto-increment primary key) on the given struct.
func (c *Commander) insertLibrary(lib *Library) error {
	// The thumbs list is stored as a JSON string in one column.
	thumbs, err := json.Marshal(lib.Thumbs)
	if err != nil {
		return err
	}
	// Insert the row with path, thumbs JSON, and the image/video counts.
	res, err := c.database.DB.Exec(
		`INSERT INTO libraries (path, thumbs, images, videos) VALUES (?, ?, ?, ?)`,
		lib.Path, string(thumbs), lib.Count.Images, lib.Count.Videos,
	)
	if err != nil {
		return err
	}
	// AUTOINCREMENT ids start at 1; store the new id in the struct.
	if id, err := res.LastInsertId(); err == nil {
		lib.ID = int(id)
	}
	return nil
}

// deleteLibrary removes the library row that matches the given folder path.
func (c *Commander) deleteLibrary(path string) error {
	_, err := c.database.DB.Exec(`DELETE FROM libraries WHERE path = ?`, path)
	return err
}

// deleteLibraryByID removes the library row with the given id.
func (c *Commander) deleteLibraryByID(id int) error {
	_, err := c.database.DB.Exec(`DELETE FROM libraries WHERE id = ?`, id)
	return err
}
