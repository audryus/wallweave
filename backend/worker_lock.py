"""Multi-process master election (flock) and cross-process wakes.

With 2 monitors the shell creates 2 widgets → 2 wallweave processes. Only
one of them may run the wallpaper workers; the other one wakes the master
process through an abstract unix socket."""

import fcntl
import socket
import sys
import threading
import time

# LOCK_PATH is the file used with flock to elect a single master process.
LOCK_PATH = "wallweave.workers.lock"
# WAKE_ADDR is an abstract unix socket (auto-cleaned on exit) used to send
# "wake" messages to the master process.
WAKE_ADDR = "\0wallweave.wake"
# RETRY_EVERY is how often (seconds) a non-master process retries to take
# the lock.
RETRY_EVERY = 5

# _lock_file holds the open lock file while this process is the master.
_lock_file = None
# is_worker_master reports whether this process currently holds the lock.
is_worker_master = False
# _listening makes sure the wake listener is started only once.
_listening = False
# _mu protects the variables above.
_mu = threading.Lock()


def try_acquire_worker_lock():
    """Tries to become the master process (the one that runs the wallpaper
    workers). Steps:
     1. If we are already the master, return True immediately.
     2. Open (or create) the lock file.
     3. Try to take an exclusive, non-blocking flock on it.
     4. If the flock succeeded, remember the file and mark us as master.
     5. If anything failed (file in use by another process), return False."""
    global _lock_file, is_worker_master
    with _mu:
        # Step 1: already the master — nothing to do.
        if is_worker_master:
            return True
        # Step 2: open or create the lock file.
        try:
            f = open(LOCK_PATH, "a+")
        except OSError:
            return False
        # Step 3: try to take the lock without blocking.
        try:
            fcntl.flock(f.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            f.close()
            return False
        # Step 4: we got the lock — remember it and mark us as master.
        _lock_file = f
        is_worker_master = True
        return True


def release_worker_lock():
    """Gives up the master lock (flock + close the file) and clears the
    master flag. Safe to call when we are not the master."""
    global _lock_file, is_worker_master
    with _mu:
        if _lock_file is not None:
            try:
                fcntl.flock(_lock_file.fileno(), fcntl.LOCK_UN)
            except OSError:
                pass
            _lock_file.close()
            _lock_file = None
        is_worker_master = False


def is_master():
    """Reports whether this process currently holds the master lock."""
    with _mu:
        return is_worker_master


def send_wake(name):
    """Notifies the master process through the abstract unix socket,
    sending the given display name (or "*" for all displays). It does
    nothing if the master is not listening."""
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as s:
            s.settimeout(1)
            s.connect(WAKE_ADDR)
            s.sendall((name + "\n").encode())
    except OSError:
        # No master listening — nothing to wake.
        pass


def listen_wake(c):
    """Accepts wake messages from other processes (only the master calls
    this). Steps:
     1. Start the listener only once.
     2. Listen on the abstract unix socket.
     3. In a background thread, accept connections forever.
     4. For each connection, read the display name and handle the wake."""
    global _listening
    # Step 1: only once per process.
    with _mu:
        if _listening:
            return
        _listening = True
    # Step 2: open the wake socket.
    try:
        ln = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        ln.bind(WAKE_ADDR)
        ln.listen()
    except OSError as e:
        print(f"[workers] listen wake: {e}", file=sys.stderr, flush=True)
        return

    def handle(conn):
        # Step 4: read the display name (up to 256 bytes) and process it.
        with conn:
            conn.settimeout(2)
            try:
                data = conn.recv(256)
            except OSError:
                data = b""
        handle_wake(c, data.decode(errors="replace").strip())

    def accept_loop():
        # Step 3: accept connections until the listener is closed.
        while True:
            try:
                conn, _ = ln.accept()
            except OSError:
                return
            threading.Thread(target=handle, args=(conn,), daemon=True).start()

    threading.Thread(target=accept_loop, daemon=True).start()


def handle_wake(c, name):
    """Runs on the master process: it makes sure the right workers exist and
    then signals them locally so they re-check their settings. A name of
    "*" (or empty) means "all displays"."""
    from . import display, worker

    # Wake all displays when the name is "*" or empty.
    if name in ("*", ""):
        worker.ensure_all_theme_workers(c)
        worker.signal_all_workers_local()
        return
    # Wake a single display: start its worker if it has a library selected.
    try:
        d = display.get_display(c, name)
    except Exception:  # noqa: BLE001
        d = None
    if d is not None and d.theme != 0:
        worker.ensure_worker(c, d)
    # Signal the local worker (if it exists in this process).
    worker.signal_worker_local(name)


def retry_worker_lock(c):
    """Keeps retrying to become the master every few seconds. If the
    current master process dies, this process takes over: it acquires the
    lock, starts the wake listener, and starts all workers."""
    from . import worker

    while True:
        time.sleep(RETRY_EVERY)
        if is_master():
            return
        # Try to take the lock from the (possibly dead) master.
        if try_acquire_worker_lock():
            listen_wake(c)
            worker.ensure_all_theme_workers(c)
            return

