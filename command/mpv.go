package command

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Every monitor gets ONE long-lived mpvpaper that shows both images and
// videos. Files are swapped through mpv's JSON IPC socket ("loadfile")
// instead of restarting mpvpaper or calling hyprpaper: both of those
// destroy and recreate a layer surface, and every layer map/unmap makes
// Hyprland send pointer events to the window under the cursor — a
// fullscreen video player (on ANY monitor) then shows its cursor and
// controls. Swapping the file keeps the same surface, so nothing is sent.

// mpvBaseOptions are the mpv options of every mpvpaper instance: no sound,
// repeat each file forever (images stay up until the next loadfile) and
// hardware decoding. Scaling uses mpv's default filter: the heavy ones
// (e.g. ewa_lanczossharp) cost GPU on every video frame for a difference
// that is hard to notice on a wallpaper.
const mpvBaseOptions = "no-audio loop-file=inf image-display-duration=inf hwdec=auto"

// mpvSocketPath returns the IPC socket path of one monitor's mpvpaper.
func mpvSocketPath(monitor string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "wallweave", "mpv-"+monitor+".sock")
}

// fitOption returns how a file fills the screen: images cover it (cropping
// the edges), videos keep their whole frame.
func fitOption(abs string) string {
	if videoExts[strings.ToLower(filepath.Ext(abs))] {
		return "panscan=0"
	}
	return "panscan=1.0"
}

// mpvShow shows a file (image or video) on a monitor. Steps:
//  1. If this monitor's mpvpaper is running, swap the file through IPC —
//     the layer surface stays the same.
//  2. Otherwise start a new mpvpaper with the file (the only moment a layer
//     surface is created: first boot or after a crash).
func mpvShow(monitor, abs string) error {
	sock := mpvSocketPath(monitor)
	// Step 1: reuse the running instance.
	if err := mpvLoadfile(sock, abs); err == nil {
		fmt.Fprintf(os.Stderr, "[mpvpaper] %s -> %s\n", monitor, filepath.Base(abs))
		return nil
	}
	// Step 2: no usable instance, start one.
	return startMpvpaper(monitor, abs, sock)
}

// mpvLoadfile replaces the file shown by a running mpvpaper. The fit option
// is passed per file, so it resets when the next file loads.
func mpvLoadfile(sock, abs string) error {
	_, err := mpvCommand(sock, map[string]string{
		"name":    "loadfile",
		"url":     abs,
		"flags":   "replace",
		"options": fitOption(abs),
	})
	return err
}

// startMpvpaper starts mpvpaper on a monitor with its IPC socket. Steps:
//  1. Stop any previous mpvpaper for this monitor (a hung instance, or one
//     started by an older wallweave without a socket) and drop a stale
//     socket file.
//  2. Start mpvpaper detached (do NOT wait on its pipes — with "-f" and
//     loop the child inherits them and Wait would block forever).
//  3. Reap the child in a background goroutine (so it does not become a
//     zombie) without blocking this worker.
//  4. Wait until the IPC socket answers, which confirms mpvpaper is up.
func startMpvpaper(monitor, abs, sock string) error {
	// Step 1: clean up whatever was there before.
	if err := stopMpvpaperForMonitor(monitor); err != nil {
		fmt.Fprintf(os.Stderr, "[mpvpaper] warning while stopping previous: %v\n", err)
	}
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return fmt.Errorf("mpvpaper socket dir: %w", err)
	}
	_ = os.Remove(sock)

	// Step 2: -p = auto-pause when hidden, -f = fork (daemonize, so it
	// survives a backend restart and the socket can be reused).
	// -l bottom keeps it above omarchy's background layer.
	opts := fmt.Sprintf("%s %s input-ipc-server=%s", mpvBaseOptions, fitOption(abs), sock)
	args := []string{"-p", "-f", "-l", "bottom", "-o", opts, monitor, abs}
	fmt.Fprintf(os.Stderr, "[mpvpaper] %s\n", strings.Join(append([]string{"mpvpaper"}, args...), " "))
	cmd := exec.Command("mpvpaper", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mpvpaper start: %w", err)
	}
	// Step 3: reap the zombie in the background.
	go func() { _ = cmd.Wait() }()

	// Step 4: wait for the socket (max ~3s).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := mpvCommand(sock, []string{"get_property", "pid"}); err == nil {
			fmt.Fprintf(os.Stderr, "[mpvpaper] started on %s -> %s\n", monitor, filepath.Base(abs))
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("mpvpaper did not open its IPC socket on %s", monitor)
}

// mpvCommand sends one command to mpv's JSON IPC socket and returns the
// "data" of its reply. Event lines mpv sends on the same connection are
// skipped.
func mpvCommand(sock string, command any) (json.RawMessage, error) {
	conn, err := net.DialTimeout("unix", sock, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	// One request per connection; the id tells our reply apart from events.
	const requestID = 1
	req, err := json.Marshal(map[string]any{"command": command, "request_id": requestID})
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var reply struct {
			RequestID *int            `json:"request_id"`
			Error     string          `json:"error"`
			Data      json.RawMessage `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &reply) != nil || reply.RequestID == nil || *reply.RequestID != requestID {
			continue
		}
		if reply.Error != "success" {
			return nil, fmt.Errorf("mpv: %s", reply.Error)
		}
		return reply.Data, nil
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("mpv closed the connection without replying")
}

// mpvpaperPids returns the pids of the mpvpaper processes running on a
// monitor. Only processes named exactly "mpvpaper" are considered, and the
// monitor must be one of their arguments — a pattern match on the full
// command line would also hit unrelated processes (e.g. a shell whose
// command mentions mpvpaper).
func mpvpaperPids(monitor string) []string {
	out, _ := exec.Command("pgrep", "-x", "mpvpaper").Output()
	var pids []string
	for pid := range strings.FieldsSeq(string(out)) {
		data, err := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if len(args) > 1 && slices.Contains(args[1:], monitor) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// stopMpvpaperForMonitor stops every mpvpaper instance running on a given
// monitor. Steps:
//  1. Find the pids.
//  2. Return immediately when nothing is running.
//  3. Kill each pid and remove the monitor's socket file.
//  4. Wait until they are gone (max ~300 ms) instead of a fixed sleep.
func stopMpvpaperForMonitor(monitor string) error {
	// Step 1: find the mpvpaper processes for this monitor.
	pids := mpvpaperPids(monitor)
	// Step 2: nothing running for this monitor.
	if len(pids) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[mpvpaper] stopping instance for %s (pids: %s)\n", monitor, strings.Join(pids, ","))
	// Step 3: kill every matching process.
	for _, pid := range pids {
		_ = exec.Command("kill", pid).Run()
	}
	_ = os.Remove(mpvSocketPath(monitor))
	// Step 4: wait until they disappear (max ~300ms).
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(mpvpaperPids(monitor)) == 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
