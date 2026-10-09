"""One long-lived mpvpaper per monitor, with file swaps over mpv's IPC.

Every monitor gets ONE long-lived mpvpaper that shows both images and
videos. Files are swapped through mpv's JSON IPC socket ("loadfile") instead
of restarting mpvpaper or calling hyprpaper: both of those destroy and
recreate a layer surface, and every layer map/unmap makes Hyprland send
pointer events to the window under the cursor — a fullscreen video player
(on ANY monitor) then shows its cursor and controls. Swapping the file
keeps the same surface, so nothing is sent."""

import json
import os
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time

from .library import VIDEO_EXTS, file_ext

# MPV_BASE_OPTIONS are the mpv options of every mpvpaper instance: no sound,
# repeat each file forever (images stay up until the next loadfile) and
# hardware decoding. Scaling uses mpv's default filter: the heavy ones
# (e.g. ewa_lanczossharp) cost GPU on every video frame for a difference
# that is hard to notice on a wallpaper.
MPV_BASE_OPTIONS = "no-audio loop-file=inf image-display-duration=inf hwdec=auto"


def _log(msg):
    print(msg, file=sys.stderr, flush=True)


def mpv_socket_path(monitor):
    """Returns the IPC socket path of one monitor's mpvpaper."""
    base = os.environ.get("XDG_RUNTIME_DIR") or tempfile.gettempdir()
    return os.path.join(base, "wallweave", "mpv-" + monitor + ".sock")


def fit_option(abs_path):
    """Returns how a file fills the screen: images cover it (cropping the
    edges), videos keep their whole frame."""
    if file_ext(abs_path).lower() in VIDEO_EXTS:
        return "panscan=0"
    return "panscan=1.0"


def mpv_show(monitor, abs_path):
    """Shows a file (image or video) on a monitor. Steps:
     1. If this monitor's mpvpaper is running, swap the file through IPC —
        the layer surface stays the same.
     2. Otherwise start a new mpvpaper with the file (the only moment a
        layer surface is created: first boot or after a crash)."""
    sock = mpv_socket_path(monitor)
    # Step 1: reuse the running instance.
    try:
        mpv_loadfile(sock, abs_path)
        _log(f"[mpvpaper] {monitor} -> {os.path.basename(abs_path)}")
        return
    except Exception:  # noqa: BLE001
        pass
    # Step 2: no usable instance, start one.
    start_mpvpaper(monitor, abs_path, sock)


def mpv_loadfile(sock, abs_path):
    """Replaces the file shown by a running mpvpaper. Steps:
     1. Load the file. The fit option is passed per file, so it resets when
        the next file loads.
     2. Unpause mpv. mpvpaper's auto-pause (-p) toggles pause every ~2s on a
        static image, and a race in it can mistake its own pause for a user
        pause and keep mpv paused forever — the next video would then freeze
        on its first frame. An explicit unpause clears that stuck state; if
        the wallpaper is really hidden, auto-pause pauses it again within
        2s."""
    # Step 1: load the file.
    mpv_command(sock, {
        "name": "loadfile",
        "url": abs_path,
        "flags": "replace",
        "options": fit_option(abs_path),
    })
    # Step 2: clear any stuck pause. The file is already loaded, so a
    # failure here is only logged — raising would make mpv_show restart
    # mpvpaper (a layer remap) for nothing.
    try:
        mpv_command(sock, ["set_property", "pause", False])
    except Exception as e:  # noqa: BLE001
        _log(f"[mpvpaper] unpause failed: {e}")


def start_mpvpaper(monitor, abs_path, sock):
    """Starts mpvpaper on a monitor with its IPC socket. Steps:
     1. Stop any previous mpvpaper for this monitor (a hung instance, or one
        started by an older wallweave without a socket) and drop a stale
        socket file.
     2. Start mpvpaper detached, with stdin/stdout/stderr on /dev/null: it
        must never inherit this process's stdin (the UI's command pipe) or
        stdout (the response pipe).
     3. Reap the child in a background thread (so it does not become a
        zombie) without blocking this worker.
     4. Wait until the IPC socket answers, which confirms mpvpaper is up."""
    # Step 1: clean up whatever was there before.
    try:
        stop_mpvpaper_for_monitor(monitor)
    except Exception as e:  # noqa: BLE001
        _log(f"[mpvpaper] warning while stopping previous: {e}")
    os.makedirs(os.path.dirname(sock), mode=0o700, exist_ok=True)
    try:
        os.remove(sock)
    except FileNotFoundError:
        pass

    # Step 2: -p = auto-pause when hidden, -f = fork (daemonize, so it
    # survives a backend restart and the socket can be reused).
    # -l bottom keeps it above omarchy's background layer.
    opts = f"{MPV_BASE_OPTIONS} {fit_option(abs_path)} input-ipc-server={sock}"
    args = ["-p", "-f", "-l", "bottom", "-o", opts, monitor, abs_path]
    _log("[mpvpaper] " + " ".join(["mpvpaper", *args]))
    try:
        proc = subprocess.Popen(
            ["mpvpaper", *args],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
    except OSError as e:
        raise RuntimeError(f"mpvpaper start: {e}") from e
    # Step 3: reap the zombie in the background.
    threading.Thread(target=proc.wait, daemon=True).start()

    # Step 4: wait for the socket (max ~3s).
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        try:
            mpv_command(sock, ["get_property", "pid"])
            _log(f"[mpvpaper] started on {monitor} -> {os.path.basename(abs_path)}")
            return
        except Exception:  # noqa: BLE001
            time.sleep(0.1)
    raise RuntimeError(f"mpvpaper did not open its IPC socket on {monitor}")


def mpv_command(sock_path, command):
    """Sends one command to mpv's JSON IPC socket and returns the "data" of
    its reply. Event lines mpv sends on the same connection are skipped."""
    # One request per connection; the id tells our reply apart from events.
    request_id = 1
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as s:
        s.settimeout(0.5)
        s.connect(sock_path)
        deadline = time.monotonic() + 2
        s.settimeout(2)
        s.sendall(json.dumps({"command": command, "request_id": request_id}).encode() + b"\n")

        buf = b""
        while True:
            # Handle every complete line already received.
            while b"\n" in buf:
                line, buf = buf.split(b"\n", 1)
                try:
                    reply = json.loads(line)
                except ValueError:
                    continue
                if not isinstance(reply, dict) or reply.get("request_id") != request_id:
                    continue
                if reply.get("error") != "success":
                    raise RuntimeError(f"mpv: {reply.get('error', '')}")
                return reply.get("data")
            # Read more, keeping the 2 second deadline for the whole call.
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError("mpv: timed out waiting for reply")
            s.settimeout(remaining)
            chunk = s.recv(65536)
            if not chunk:
                raise ConnectionError("mpv closed the connection without replying")
            buf += chunk


def mpvpaper_pids(monitor):
    """Returns the pids of the mpvpaper processes running on a monitor.
    Only processes named exactly "mpvpaper" are considered, and the monitor
    must be one of their arguments — a pattern match on the full command
    line would also hit unrelated processes (e.g. a shell whose command
    mentions mpvpaper)."""
    try:
        out = subprocess.run(
            ["pgrep", "-x", "mpvpaper"], stdin=subprocess.DEVNULL, capture_output=True, text=True
        ).stdout
    except OSError:
        return []
    pids = []
    for pid in out.split():
        try:
            with open(os.path.join("/proc", pid, "cmdline"), "rb") as f:
                data = f.read()
        except OSError:
            continue
        args = data.rstrip(b"\x00").decode(errors="replace").split("\x00")
        if len(args) > 1 and monitor in args[1:]:
            pids.append(pid)
    return pids


def stop_mpvpaper_for_monitor(monitor):
    """Stops every mpvpaper instance running on a given monitor. Steps:
     1. Find the pids.
     2. Return immediately when nothing is running.
     3. Kill each pid and remove the monitor's socket file.
     4. Wait until they are gone (max ~300 ms) instead of a fixed sleep."""
    # Step 1 and 2: find the processes, nothing to do when none.
    pids = mpvpaper_pids(monitor)
    if not pids:
        return
    _log(f"[mpvpaper] stopping instance for {monitor} (pids: {','.join(pids)})")
    # Step 3: kill every matching process.
    for pid in pids:
        try:
            os.kill(int(pid), signal.SIGTERM)
        except OSError:
            pass
    try:
        os.remove(mpv_socket_path(monitor))
    except FileNotFoundError:
        pass
    # Step 4: wait until they disappear (max ~300ms).
    deadline = time.monotonic() + 0.3
    while time.monotonic() < deadline:
        if not mpvpaper_pids(monitor):
            return
        time.sleep(0.05)
