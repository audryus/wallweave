"""Wallpaper workers: one rotation loop per display, plus the restore to
Omarchy's default wallpaper."""

import os
import queue
import random
import subprocess
import sys
import threading
import time

from . import display, mpv, worker_lock
from .library import IMAGE_EXTS, VIDEO_EXTS, fetch_libraries, file_ext
from .status import load_status, probe_status, save_status

# _wakes holds one wake queue per running worker, so we can tell a worker to
# re-check its settings immediately (instead of waiting for the timer). A
# display has a running worker exactly when it has an entry here.
_wakes = {}
# _mu protects _wakes.
_mu = threading.Lock()


def _log(msg):
    print(msg, file=sys.stderr, flush=True)


def ensure_worker(c, d):
    """Starts a wallpaper worker for a display, but only if:
     1. This process is the master (holds the flock), and
     2. A worker for that display is not already running.
    It creates the wake queue, marks the display as running, and launches
    the worker thread."""
    if not worker_lock.is_master():
        return
    with _mu:
        if d.name in _wakes:
            return
        # maxsize=1: a pending wake is never lost and never blocks the
        # sender (same as Go's buffered channel of size 1).
        wake = queue.Queue(maxsize=1)
        _wakes[d.name] = wake
    threading.Thread(target=run_worker, args=(c, d.name, wake), daemon=True, name="worker-" + d.name).start()


def start_workers(c):
    """Starts the wallpaper workers when the application boots. Only the
    master process (the one that gets the flock) starts them; the other
    processes go into a retry loop and take over if the master dies."""
    if worker_lock.try_acquire_worker_lock():
        worker_lock.listen_wake(c)
        ensure_all_theme_workers(c)
        return
    threading.Thread(target=worker_lock.retry_worker_lock, args=(c,), daemon=True).start()


def ensure_all_theme_workers(c):
    """Starts a worker for every display that has a library selected
    (theme != 0). Displays with theme 0 use the default wallpaper and need
    no worker."""
    try:
        displays = display.list_displays(c)
    except Exception:  # noqa: BLE001
        return
    for d in displays:
        if d.theme != 0:
            ensure_worker(c, d)


def _poke(wake):
    """Sends a wake without blocking (skipped when one is already pending)."""
    try:
        wake.put_nowait(None)
    except queue.Full:
        pass


def signal_worker(name):
    """Wakes one worker (used when the theme was edited in the UI).
    Cross-process: if there is no local worker for that display (it lives in
    another process), a wake message is sent to the master."""
    if signal_worker_local(name):
        return
    if not worker_lock.is_master():
        worker_lock.send_wake(name)


def signal_all_workers():
    """Wakes every worker (used for example when a library is deleted),
    falling back to a cross-process wake like signal_worker."""
    if signal_all_workers_local():
        return
    if not worker_lock.is_master():
        worker_lock.send_wake("*")


def signal_worker_local(name):
    """Wakes one worker of this process. Returns False when no worker exists
    for that display name."""
    with _mu:
        wake = _wakes.get(name)
    if wake is None:
        return False
    _poke(wake)
    return True


def signal_all_workers_local():
    """Wakes every worker of this process. Returns False when there are no
    workers at all."""
    with _mu:
        wakes = list(_wakes.values())
    if not wakes:
        return False
    for wake in wakes:
        _poke(wake)
    return True


def run_worker(c, name, wake):
    """Main loop of one display's wallpaper worker. It runs until the
    display disappears from the database, and logs (never dies on) any other
    error. Every tick it:
     1. Reads the display settings from the database.
     2. Waits for the timer OR a wake signal from the UI.
     3. Re-reads the settings (so UI edits are picked up).
     4. If theme is 0, restores the default wallpaper (once).
     5. Otherwise picks the next file from the selected library (shuffled,
        wrapping around) and applies it (image or video)."""
    state = {
        # 0 = omarchy default already considered "applied" — avoids a
        # restore on the first boot.
        "last_theme": 0,
        # Index of the next file to apply (keeps increasing; we use modulo).
        "current": 0,
        # How many consecutive ticks the display was missing.
        "missing": 0,
        # The first tick applies immediately, without waiting.
        "first": True,
    }
    try:
        while True:
            try:
                if not _tick(c, name, wake, state):
                    return
            except Exception as e:  # noqa: BLE001
                # Unexpected error (e.g. database): wait and try again.
                _log(f"[worker {name}] {e}")
                time.sleep(5)
    finally:
        # When the worker exits, forget it so it can be started again.
        with _mu:
            if _wakes.get(name) is wake:
                del _wakes[name]


def _tick(c, name, wake, state):
    """Runs one iteration of run_worker. Returns False when the worker must
    stop."""
    # Step 1: read the display settings.
    d = display.get_display(c, name)
    if d is None:
        # Display removed → shut down after 3 consecutive misses (avoids
        # dying on a transient error).
        state["missing"] += 1
        if state["missing"] >= 3:
            return False
        time.sleep(5)
        return True
    state["missing"] = 0

    # Step 2: wait for the timer OR a wake from the UI. The first tick
    # applies right away; later ticks wait.
    if not state["first"]:
        timer = d.timer if d.timer >= 1 else 60
        try:
            wake.get(timeout=timer)
        except queue.Empty:
            pass
    state["first"] = False

    # Step 3: re-read so theme/timer changes made in the UI are reflected.
    d = display.get_display(c, name)
    if d is None:
        return True

    # Step 4: theme 0 means "use the omarchy default wallpaper".
    if d.theme == 0:
        if state["last_theme"] != 0:
            _restore_quietly(d.name)
            state["last_theme"] = 0
            state["current"] = 0
        return True

    # When the selected library changes, restart the file rotation.
    if state["last_theme"] != d.theme:
        state["current"] = 0
        state["last_theme"] = d.theme

    # Load the status (mpvpaper availability). If it is not in the database
    # yet, probe the system and save it.
    try:
        status = load_status(c)
    except Exception:  # noqa: BLE001
        status = probe_status()
        try:
            save_status(c, status)
        except Exception:  # noqa: BLE001
            pass

    # Map library id → folder path.
    lib_path = map_libraries(c).get(d.theme, "")
    if lib_path == "":
        # The library was deleted — restore the default as a defensive
        # fallback.
        _restore_quietly(d.name)
        state["last_theme"] = 0
        state["current"] = 0
        return True

    # Usable wallpaper files in that library (shuffled).
    files = list_wallpaper_files(d, lib_path)
    if not files:
        return True

    # Pick the next file with infinite wrap-around.
    file = files[state["current"] % len(files)]
    state["current"] += 1

    # Apply the wallpaper: images and videos both go to this monitor's
    # long-lived mpvpaper. Without mpvpaper, images use Omarchy's global
    # background and videos cannot play.
    try:
        if status.mpvpaper:
            mpv.mpv_show(d.name, file)
        elif file_ext(file).lower() in VIDEO_EXTS:
            raise RuntimeError(f"mpvpaper is not installed, cannot play {os.path.basename(file)}")
        else:
            set_omarchy_background(file)
    except Exception as e:  # noqa: BLE001
        # Log the failure but keep the worker running.
        _log(f"[worker {name}] {e}")
    return True


def _restore_quietly(monitor):
    """restore_omarchy_default, logging instead of raising."""
    try:
        restore_omarchy_default(monitor)
    except Exception as e:  # noqa: BLE001
        _log(f"[undo {monitor}] {e}")


def list_wallpaper_files(d, root):
    """Returns the usable wallpaper files under a library folder, in random
    order. Steps:
     1. Return nothing when the path is empty.
     2. Walk the folder, skipping the "thumbs" directory and unreadable
        entries.
     3. Keep images always; keep videos only when d.video is true.
     4. Shuffle the list with the display's seed, so each monitor has its
        own stable random order."""
    # Step 1: empty path → no files.
    if root == "":
        return None
    # Step 2 and 3: walk in lexical order (the shuffle needs a stable input).
    files = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames.sort()
        if dirpath == root and "thumbs" in dirnames:
            dirnames.remove("thumbs")
        for fname in sorted(filenames):
            ext = file_ext(fname).lower()
            if ext in IMAGE_EXTS or (d.video and ext in VIDEO_EXTS):
                files.append(os.path.join(dirpath, fname))
    # Step 4: shuffle with the display's seed.
    random.Random(d.seed).shuffle(files)
    return files


def _quiet(args):
    """Runs a command, ignoring its output and exit status."""
    try:
        subprocess.run(args, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    except OSError:
        pass


def set_omarchy_background(abs_path):
    """Sets Omarchy's global background (shared by all monitors). It is the
    image fallback when mpvpaper is not installed: omarchy-shell swaps the
    image on its existing layer surface, so no pointer events are sent."""
    # The command's own output goes to our stderr (never to stdout, which
    # carries the JSON responses).
    try:
        rc = subprocess.run(
            ["omarchy", "theme", "bg", "set", abs_path],
            stdin=subprocess.DEVNULL,
            stdout=sys.stderr,
            stderr=sys.stderr,
        ).returncode
    except OSError as e:
        raise RuntimeError(f"omarchy theme bg set failed: {e}") from e
    if rc != 0:
        raise RuntimeError(f"omarchy theme bg set failed: exit status {rc}")
    # Also notify omarchy-shell so its UI picks up the new background.
    _quiet(["omarchy-shell", "-q", "background", "set", abs_path])


def map_libraries(c):
    """Returns a map of library id → folder path."""
    return {lib.id: lib.path for lib in fetch_libraries(c)}


def restore_omarchy_default(monitor):
    """Goes back to the global omarchy wallpaper, but only for this monitor.
    Steps:
     1. Find the current omarchy theme name.
     2. Find the default wallpaper image of that theme.
     3. Set the default wallpaper BEFORE killing the video (avoids a flash
        of the previous wallpaper). Prefer the omarchy command; if it fails,
        create the symlink manually as a fallback.
     4. Stop mpvpaper for THIS monitor only (never touch the others). This
        remaps a layer once, but only on a user action — keeping mpvpaper up
        would hide later omarchy theme changes."""
    # Step 1: current theme name.
    theme_name = get_current_theme_name()
    _log(f"[undo {monitor}] current theme: {theme_name}")

    # Step 2: default wallpaper for that theme.
    default_path = find_default_wallpaper(theme_name)
    if default_path == "":
        raise RuntimeError(f"no default wallpaper found for theme {theme_name}")
    _log(f"[undo {monitor}] default wallpaper: {default_path}")

    # Step 3: omarchy global (shared background).
    try:
        set_omarchy_background(default_path)
    except RuntimeError as e:
        _log(f"[warn] {e}, trying manual symlink")
        link = os.path.join(os.path.expanduser("~"), ".local/state/omarchy/current/background")
        os.makedirs(os.path.dirname(link), mode=0o755, exist_ok=True)
        try:
            os.remove(link)
        except FileNotFoundError:
            pass
        try:
            os.symlink(default_path, link)
        except OSError as err:
            raise RuntimeError(f"failed to create symlink {link} -> {default_path}: {err}") from err
        _log(f"[undo {monitor}] symlink {link} -> {default_path}")
        _quiet(["omarchy-shell", "-q", "background", "set", default_path])

    # Step 4: give the compositor one frame, then stop this monitor's
    # mpvpaper to reveal the default.
    time.sleep(0.05)
    try:
        mpv.stop_mpvpaper_for_monitor(monitor)
    except Exception:  # noqa: BLE001
        pass


def _output(args):
    """Returns a command's stdout, or None when it fails."""
    try:
        proc = subprocess.run(args, stdin=subprocess.DEVNULL, capture_output=True, text=True)
    except OSError:
        return None
    return proc.stdout if proc.returncode == 0 else None


def get_current_theme_name():
    """Returns the name of the active omarchy theme. Steps:
     1. Read ~/.local/state/omarchy/current/theme.name.
     2. Otherwise ask "omarchy theme current" and normalize it (lowercase,
        spaces → dashes).
     3. If everything fails, return the default "tokyo-night"."""
    # Step 1: the state file written by omarchy.
    try:
        with open(os.path.join(os.environ.get("HOME", ""), ".local/state/omarchy/current/theme.name")) as f:
            name = f.read().strip()
        if name:
            return name
    except OSError:
        pass
    # Step 2: ask omarchy.
    out = _output(["omarchy", "theme", "current"])
    if out and out.strip():
        return out.strip().lower().replace(" ", "-")
    # Step 3: default theme name.
    return "tokyo-night"


def find_default_wallpaper(theme_name):
    """Looks for the default wallpaper image of a theme. Steps:
     1. Build the candidate folders (omarchy state dir, user config, system
        themes dir), with the folder from "omarchy theme dir" first.
     2. For each folder, collect image files (including .bmp).
     3. Return the first image (sorted) of the first folder that has any,
        or "" when none is found."""
    home = os.environ.get("HOME", "")
    # Step 1: candidate folders, from most to least specific.
    candidates = [
        os.path.join(home, ".local/state/omarchy/current/theme/backgrounds"),
        os.path.join(home, ".config/omarchy/backgrounds", theme_name),
        os.path.join("/usr/share/omarchy/themes", theme_name, "backgrounds"),
    ]
    out = _output(["omarchy", "theme", "dir", theme_name])
    if out and out.strip():
        candidates.insert(0, os.path.join(out.strip(), "backgrounds"))

    # Step 2 and 3: first image of the first folder that has any.
    for folder in candidates:
        try:
            entries = list(os.scandir(folder))
        except OSError:
            continue
        files = sorted(
            os.path.join(folder, e.name)
            for e in entries
            if not e.is_dir()
            and (file_ext(e.name).lower() in IMAGE_EXTS or e.name.lower().endswith(".bmp"))
        )
        if files:
            return files[0]
    return ""
