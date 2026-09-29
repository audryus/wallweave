package command

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
)

// fakeMpv listens on a unix socket and answers each request like mpv does:
// first an unrelated event line, then the reply for the request_id. It
// sends every received command to the returned channel.
func fakeMpv(t *testing.T, reply string) (string, <-chan map[string]any) {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "mpv.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	got := make(chan map[string]any, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			sc := bufio.NewScanner(conn)
			if sc.Scan() {
				var req map[string]any
				_ = json.Unmarshal(sc.Bytes(), &req)
				got <- req
				_, _ = conn.Write([]byte(`{"event":"start-file","playlist_entry_id":2}` + "\n"))
				_, _ = conn.Write([]byte(reply + "\n"))
			}
			conn.Close()
		}
	}()
	return sock, got
}

// TestMpvLoadfile checks that loadfile sends the named-argument command
// with the fit option (skipping mpv's event lines to read the reply), then
// unpauses mpv to clear a stuck auto-pause.
func TestMpvLoadfile(t *testing.T) {
	sock, got := fakeMpv(t, `{"data":{"playlist_entry_id":2},"request_id":1,"error":"success"}`)

	if err := mpvLoadfile(sock, "/walls/clip.MP4"); err != nil {
		t.Fatalf("loadfile: %v", err)
	}
	req := <-got
	cmd, _ := req["command"].(map[string]any)
	if cmd["name"] != "loadfile" || cmd["url"] != "/walls/clip.MP4" || cmd["flags"] != "replace" || cmd["options"] != "panscan=0" {
		t.Errorf("unexpected command: %v", req)
	}
	unpause, _ := (<-got)["command"].([]any)
	if len(unpause) != 3 || unpause[0] != "set_property" || unpause[1] != "pause" || unpause[2] != false {
		t.Errorf("expected unpause after loadfile, got %v", unpause)
	}
}

// TestMpvCommandError checks that an mpv error reply becomes a Go error.
func TestMpvCommandError(t *testing.T) {
	sock, _ := fakeMpv(t, `{"request_id":1,"error":"property unavailable"}`)
	if _, err := mpvCommand(sock, []string{"get_property", "pid"}); err == nil {
		t.Fatal("expected an error")
	}
}

// TestMpvCommandNoSocket checks that a missing socket fails fast (this is
// how mpvShow knows it must start mpvpaper).
func TestMpvCommandNoSocket(t *testing.T) {
	if _, err := mpvCommand(filepath.Join(t.TempDir(), "none.sock"), []string{"get_property", "pid"}); err == nil {
		t.Fatal("expected an error")
	}
}

// TestFitOption checks that images cover the screen and videos keep their
// whole frame.
func TestFitOption(t *testing.T) {
	cases := map[string]string{
		"/a/b.jpg":  "panscan=1.0",
		"/a/b.PNG":  "panscan=1.0",
		"/a/b.webm": "panscan=0",
		"/a/b.mkv":  "panscan=0",
	}
	for path, want := range cases {
		if got := fitOption(path); got != want {
			t.Errorf("fitOption(%q) = %q, want %q", path, got, want)
		}
	}
}
