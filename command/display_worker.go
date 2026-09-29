package command

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// running tracks which displays already have a worker goroutine.
	running = make(map[string]bool)
	// wakes holds one wake channel per running worker, so we can tell a
	// worker to re-check its settings immediately (instead of waiting for
	// the timer).
	wakes = make(map[string]chan struct{})
	// workerMu protects the running and wakes maps.
	workerMu sync.Mutex
)

// imageExts lists the file extensions treated as images (lowercase).
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// videoExts lists the file extensions treated as videos (lowercase).
var videoExts = map[string]bool{
	".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".webm": true,
}

// ensureWorker starts a wallpaper worker for a display, but only if:
//  1. This process is the master (holds the flock), and
//  2. A worker for that display is not already running.
//
// It creates the wake channel, marks the display as running, and launches
// the worker goroutine.
func (c *Commander) ensureWorker(d Display) {
	// Only the master process runs workers.
	if !isWorkerMaster {
		return
	}
	// Protect the shared maps.
	workerMu.Lock()
	defer workerMu.Unlock()
	// Do not start a second worker for the same display.
	if running[d.Name] {
		return
	}
	// Mark as running and create its wake channel (buffer size 1 so a
	// pending wake is never lost and never blocks the sender).
	running[d.Name] = true
	ch := make(chan struct{}, 1)
	wakes[d.Name] = ch
	// Start the worker in the background.
	go c.runWorker(d.Name, ch)
}

// StartWorkers starts the wallpaper workers when the application boots.
// Only the master process (the one that gets the flock) starts them; the
// other processes go into a retry loop and take over if the master dies.
// Called at application boot — it does not wait for the UI.
func (c *Commander) StartWorkers() {
	// Try to become the master right away.
	if tryAcquireWorkerLock() {
		// We are the master: listen for wakes and start all workers.
		c.listenWake()
		c.ensureAllThemeWorkers()
		return
	}
	// Another process is the master: retry in the background until it dies.
	go c.retryWorkerLock()
}

// ensureAllThemeWorkers starts a worker for every display that has a
// library selected (theme != 0). Displays with theme 0 use the default
// wallpaper and need no worker.
func (c *Commander) ensureAllThemeWorkers() {
	// Load all displays from the database.
	displays, err := c.listDisplays()
	if err != nil {
		return
	}
	// Start a worker only for displays that have a library selected.
	for _, d := range displays {
		if d.Theme != 0 {
			c.ensureWorker(d)
		}
	}
}

// signalWorker wakes one worker (used when the timer or theme was edited
// in the UI).
// Cross-process: if there is no local channel for that display (the worker
// lives in another process), a wake message is sent to the master through
// the unix socket.
func signalWorker(name string) {
	// Try the local (same-process) channel first.
	if signalWorkerLocal(name) {
		return
	}
	// No local worker: if we are not the master, ask the master to wake it.
	if !isWorkerMaster {
		sendWake(name)
	}
}

// signalAllWorkers wakes every worker (used for example when a library is
// deleted). Like signalWorker, it falls back to a cross-process wake when
// the workers live in another process.
func signalAllWorkers() {
	// Try to wake all local workers first.
	if signalAllWorkersLocal() {
		return
	}
	// No local workers: ask the master to wake all of them ("*").
	if !isWorkerMaster {
		sendWake("*")
	}
}

// signalWorkerLocal sends a wake signal to one worker in this process.
// It returns false when no worker exists for that display name.
// The send is non-blocking: if a wake is already pending, it is skipped.
func signalWorkerLocal(name string) bool {
	// Look up the wake channel for this display.
	workerMu.Lock()
	ch := wakes[name]
	workerMu.Unlock()
	if ch == nil {
		return false
	}
	// Send the wake without blocking (the channel buffer may be full).
	select {
	case ch <- struct{}{}:
	default:
	}
	return true
}

// signalAllWorkersLocal sends a wake signal to every worker in this
// process. It returns false when there are no workers at all.
func signalAllWorkersLocal() bool {
	// Copy all wake channels under the lock.
	workerMu.Lock()
	chs := make([]chan struct{}, 0, len(wakes))
	for _, ch := range wakes {
		chs = append(chs, ch)
	}
	workerMu.Unlock()
	// Nothing to wake.
	if len(chs) == 0 {
		return false
	}
	// Wake each worker without blocking.
	for _, ch := range chs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	return true
}

// runWorker is the main loop of one display's wallpaper worker. It runs
// forever until the display disappears from the database. Every tick it:
//  1. Reads the display settings from the database.
//  2. Waits for the timer OR a wake signal from the UI.
//  3. Re-reads the settings (so UI edits are picked up).
//  4. If theme is 0, restores the default wallpaper (once).
//  5. Otherwise picks the next file from the selected library (shuffled,
//     wrapping around) and applies it (image or video).
func (c *Commander) runWorker(name string, wake <-chan struct{}) {
	// When the worker exits, remove it from the running/wakes maps.
	defer func() {
		workerMu.Lock()
		delete(running, name)
		delete(wakes, name)
		workerMu.Unlock()
	}()

	// State kept between ticks:
	// 0 = omarchy default already considered "applied" — avoids a restore
	// on the first boot.
	lastAppliedTheme := 0
	// Index of the next file to apply (keeps increasing; we use modulo).
	currentFile := 0
	// How many consecutive ticks the display was missing from the database.
	missing := 0
	// The first tick applies immediately, without waiting.
	first := true

	// Main loop: runs until the display is gone.
	for {
		// Step 1: read the display settings from the database.
		d, ok, err := c.getDisplay(name)
		if err != nil {
			// Database error: wait and try again.
			time.Sleep(5 * time.Second)
			continue
		}
		if !ok {
			// Display removed from the database → shut this worker down
			// after 3 consecutive misses (avoids dying on a transient error).
			missing++
			if missing >= 3 {
				return
			}
			time.Sleep(5 * time.Second)
			continue
		}
		// The display exists: reset the miss counter.
		missing = 0

		// Step 2: wait for the timer OR a wake from the UI. The first tick
		// applies right away; later ticks wait.
		if !first {
			// Default to 60 seconds when the timer is invalid.
			timer := d.Timer
			if timer < 1 {
				timer = 60
			}
			// Wait for the timer to expire or for a wake signal.
			timerC := time.After(time.Duration(timer) * time.Second)
			select {
			case <-timerC:
			case <-wake:
			}
		}
		first = false

		// Step 3: re-read the settings on every tick so theme/timer changes
		// made in the UI are reflected immediately.
		d, ok, err = c.getDisplay(name)
		if err != nil || !ok {
			continue
		}

		// Step 4: theme 0 means "use the omarchy default wallpaper".
		if d.Theme == 0 {
			// Restore only if we previously applied a library theme.
			if lastAppliedTheme != 0 {
				_ = c.restoreOmarchyDefault(d.Name)
				lastAppliedTheme = 0
				currentFile = 0
			}
			continue
		}

		// When the selected library changes, restart the file rotation.
		if lastAppliedTheme != d.Theme {
			currentFile = 0
			lastAppliedTheme = d.Theme
		}

		// Load the status (mpvpaper availability). If it is not
		// in the database yet, probe the system and save it.
		status, err := c.loadStatus()
		if err != nil {
			status = probeStatus()
			_ = c.saveStatus(status)
		}

		// Map library id → folder path.
		libs, err := c.mapLibraries()
		if err != nil {
			continue
		}
		libPath := libs[d.Theme]
		if libPath == "" {
			// The library was deleted — the outbox should set theme=0, but
			// restore the default here as a defensive fallback.
			_ = c.restoreOmarchyDefault(d.Name)
			lastAppliedTheme = 0
			currentFile = 0
			continue
		}

		// List the usable wallpaper files in that library (shuffled).
		files := listWallpaperFiles(d, libPath)
		if len(files) == 0 {
			// Nothing to show in this library.
			continue
		}

		// Pick the next file with infinite wrap-around — the worker never
		// gets stuck after the last file.
		file := files[currentFile%len(files)]
		currentFile++

		// Apply the wallpaper: images and videos both go to this monitor's
		// long-lived mpvpaper (see mpv.go). Without mpvpaper, images use
		// Omarchy's global background and videos cannot play.
		ext := strings.ToLower(filepath.Ext(file))
		var applyErr error
		switch {
		case status.Mpvpaper:
			applyErr = mpvShow(d.Name, file)
		case videoExts[ext]:
			applyErr = fmt.Errorf("mpvpaper is not installed, cannot play %s", filepath.Base(file))
		default:
			applyErr = setOmarchyBackground(file)
		}
		if applyErr != nil {
			// Log the failure but keep the worker running.
			fmt.Fprintf(os.Stderr, "[worker %s] %v\n", name, applyErr)
		}
	}
}

// listWallpaperFiles returns the usable wallpaper files under a library
// folder, in random order.
// Steps:
//  1. Return nothing when the path is empty.
//  2. Walk the folder, skipping the "thumbs" directory.
//  3. Keep images always; keep videos only when display.Video is true
//     (includeVideos=false when video is turned off).
//  4. Shuffle the list with the display's seed, so each monitor has its
//     own stable random order.
func listWallpaperFiles(display Display, root string) []string {
	// Videos are included only when the display allows them.
	includeVideos := display.Video

	// Step 1: empty path → no files.
	if root == "" {
		return nil
	}
	// Step 2: walk the folder, skipping thumbs/.
	thumbsDir := filepath.Join(root, "thumbs")
	var files []string
	// WalkDir reads only directory entries (no stat per file), which is
	// several times faster than Walk on big libraries.
	_ = filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Ignore unreadable entries and keep walking.
			return nil
		}
		if entry.IsDir() {
			// Skip the thumbs directory entirely.
			if p == thumbsDir {
				return filepath.SkipDir
			}
			return nil
		}
		// Safety: ignore files that live inside thumbs/.
		if filepath.Dir(p) == thumbsDir {
			return nil
		}
		// Step 3: keep the file only if its extension is supported.
		ext := strings.ToLower(filepath.Ext(p))
		if imageExts[ext] {
			files = append(files, p)
			return nil
		}
		if includeVideos && videoExts[ext] {
			files = append(files, p)
		}
		return nil
	})

	// Step 4: shuffle the list using the display's seed, so the order is
	// random but stable for this monitor.
	src := rand.NewSource(display.Seed)
	r := rand.New(src)
	r.Shuffle(len(files), func(i, j int) {
		files[i], files[j] = files[j], files[i]
	})

	return files
}

// setOmarchyBackground sets Omarchy's global background (shared by all
// monitors). It is the image fallback when mpvpaper is not installed:
// omarchy-shell swaps the image on its existing layer surface, so no
// pointer events are sent.
func setOmarchyBackground(abs string) error {
	cmd := exec.Command("omarchy", "theme", "bg", "set", abs)
	// Send the command's own output to our stderr for debugging.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("omarchy theme bg set failed: %w", err)
	}
	// Also notify omarchy-shell so its UI picks up the new background.
	_ = exec.Command("omarchy-shell", "-q", "background", "set", abs).Run()
	return nil
}

// mapLibraries loads all libraries from the database and returns a map of
// library id → folder path, for quick lookup by the workers.
func (c *Commander) mapLibraries() (map[int]string, error) {
	// Read every library row.
	libs, err := c.fetchLibraries()
	if err != nil {
		return nil, err
	}
	// Build the id → path map.
	mapa := make(map[int]string, len(libs))
	for i := range libs {
		mapa[libs[i].ID] = libs[i].Path
	}
	return mapa, nil
}

// restoreOmarchyDefault goes back to the global omarchy wallpaper, but
// only for this monitor. Steps:
//  1. Find the current omarchy theme name.
//  2. Find the default wallpaper image of that theme.
//  3. Set the default wallpaper BEFORE killing the video (avoids a flash of
//     the previous wallpaper). Prefer the omarchy command; if it fails,
//     create the symlink manually as a fallback.
//  4. Stop mpvpaper for THIS monitor only (never touch the others). This
//     remaps a layer once, but only on a user action — keeping mpvpaper up
//     would hide later omarchy theme changes.
func (c *Commander) restoreOmarchyDefault(monitor string) error {
	// Step 1: get the current theme name.
	themeName := getCurrentThemeName()
	fmt.Fprintf(os.Stderr, "[undo %s] current theme: %s\n", monitor, themeName)

	// Step 2: locate the default wallpaper for that theme.
	defaultPath := findDefaultWallpaper(themeName)
	if defaultPath == "" {
		return fmt.Errorf("no default wallpaper found for theme %s", themeName)
	}
	fmt.Fprintf(os.Stderr, "[undo %s] default wallpaper: %s\n", monitor, defaultPath)

	// Step 3: omarchy global (shared background).
	if err := setOmarchyBackground(defaultPath); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] %v, trying manual symlink\n", err)
		// Fallback: create the symlink manually.
		home, _ := os.UserHomeDir()
		link := filepath.Join(home, ".local/state/omarchy/current/background")
		// Make sure the folder exists, remove any old link, create the new one.
		_ = os.MkdirAll(filepath.Dir(link), 0o755)
		_ = os.Remove(link)
		if err := os.Symlink(defaultPath, link); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", link, defaultPath, err)
		}
		fmt.Fprintf(os.Stderr, "[undo %s] symlink %s -> %s\n", monitor, link, defaultPath)
		// Notify omarchy-shell about the new background.
		_ = exec.Command("omarchy-shell", "-q", "background", "set", defaultPath).Run()
	}

	// Step 4: give the compositor one frame, then stop this monitor's
	// mpvpaper to reveal the default.
	time.Sleep(50 * time.Millisecond)
	_ = stopMpvpaperForMonitor(monitor)
	return nil
}

// getCurrentThemeName returns the name of the active omarchy theme.
// Steps:
//  1. Try reading the theme name from the state file
//     (~/.local/state/omarchy/current/theme.name).
//  2. If that fails, ask omarchy itself with "omarchy theme current" and
//     normalize it (lowercase, spaces → dashes).
//  3. If everything fails, return the default "tokyo-night".
func getCurrentThemeName() string {
	// Step 1: read the state file written by omarchy.
	if data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".local/state/omarchy/current/theme.name")); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			return name
		}
	}
	// Step 2: ask omarchy for the current theme and normalize the name.
	if out, err := exec.Command("omarchy", "theme", "current").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			lower := strings.ToLower(name)
			lower = strings.ReplaceAll(lower, " ", "-")
			return lower
		}
	}
	// Step 3: give up and use the default theme name.
	return "tokyo-night"
}

// findDefaultWallpaper looks for the default wallpaper image of a theme.
// Steps:
//  1. Build a list of candidate folders (omarchy state dir, user config,
//     system themes dir) and ask omarchy for the theme dir (prepended as
//     the first candidate when available).
//  2. For each candidate folder, collect image files (including .bmp).
//  3. Return the first image of the first folder that has any (sorted
//     alphabetically for a stable choice). Return "" if none is found.
func findDefaultWallpaper(themeName string) string {
	home := os.Getenv("HOME")
	// Step 1: candidate folders, from most to least specific.
	candidates := []string{
		filepath.Join(home, ".local/state/omarchy/current/theme/backgrounds"),
		filepath.Join(home, ".config/omarchy/backgrounds", themeName),
		filepath.Join("/usr/share/omarchy/themes", themeName, "backgrounds"),
	}
	// Ask omarchy where the theme lives; put that folder first.
	if out, err := exec.Command("omarchy", "theme", "dir", themeName).Output(); err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			candidates = append([]string{filepath.Join(dir, "backgrounds")}, candidates...)
		}
	}

	// Step 2: try each candidate folder in order.
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			// Folder does not exist or cannot be read — try the next one.
			continue
		}
		// Collect image files from this folder.
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				// Skip subdirectories.
				continue
			}
			low := strings.ToLower(e.Name())
			// Keep known image extensions plus .bmp.
			if imageExts[filepath.Ext(low)] || strings.HasSuffix(low, ".bmp") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
		// Step 3: return the first image (sorted) of the first folder that
		// has any image.
		if len(files) > 0 {
			sort.Strings(files)
			return files[0]
		}
	}
	// No default wallpaper found anywhere.
	return ""
}
