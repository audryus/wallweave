package command

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// With 2 monitors the shell creates 2 widgets → 2 wallweave processes.
// Only one of them may run the wallpaper workers; the other one wakes the
// master process through an abstract unix socket.
const (
	// lockPath is the file used with flock to elect a single master process.
	lockPath = "wallweave.workers.lock"
	// wakeAddr is an abstract unix socket (auto-cleaned on exit) used to
	// send "wake" messages to the master process.
	wakeAddr = "\x00wallweave.wake"
	// retryEvery is how often a non-master process retries to take the lock.
	retryEvery = 5 * time.Second
)

var (
	// lockFile holds the open lock file while this process is the master.
	lockFile *os.File
	// isWorkerMaster reports whether this process currently holds the lock.
	isWorkerMaster bool
	// wakeListenOnce makes sure the wake listener is started only once.
	wakeListenOnce sync.Once
	// wakeMu protects lockFile and isWorkerMaster.
	wakeMu sync.Mutex
)

// tryAcquireWorkerLock tries to become the master process (the one that
// runs the wallpaper workers). Steps:
//  1. If we are already the master, return true immediately.
//  2. Open (or create) the lock file.
//  3. Try to take an exclusive, non-blocking flock on it.
//  4. If the flock succeeded, remember the file and mark us as master.
//  5. If anything failed (file in use by another process), return false.
func tryAcquireWorkerLock() bool {
	// Guard the shared variables.
	wakeMu.Lock()
	defer wakeMu.Unlock()
	// Step 1: already the master — nothing to do.
	if isWorkerMaster {
		return true
	}
	// Step 2: open or create the lock file.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false
	}
	// Step 3: try to take the lock without blocking. If another process
	// already holds it, flock returns an error immediately.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}
	// Step 4: we got the lock — remember it and mark us as master.
	lockFile = f
	isWorkerMaster = true
	return true
}

// releaseWorkerLock gives up the master lock (flock + close the file) and
// clears the master flag. Safe to call when we are not the master.
func releaseWorkerLock() {
	// Guard the shared variables.
	wakeMu.Lock()
	defer wakeMu.Unlock()
	if lockFile != nil {
		// Release the flock and close the file handle.
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		lockFile = nil
	}
	// We are no longer the master.
	isWorkerMaster = false
}

// sendWake notifies the master process through the abstract unix socket,
// sending the given display name (or "*" for all displays). It does
// nothing if the master is not listening.
func sendWake(name string) {
	// Connect to the master's wake socket.
	conn, err := net.Dial("unix", wakeAddr)
	if err != nil {
		// No master listening — nothing to wake.
		return
	}
	// Close the connection when this function returns.
	defer conn.Close()
	// Send the display name followed by a newline.
	_, _ = conn.Write([]byte(name + "\n"))
}

// listenWake accepts wake messages from other processes (only the master
// process calls this). Steps:
//  1. Start the listener only once (sync.Once).
//  2. Listen on the abstract unix socket.
//  3. In a background goroutine, accept connections forever.
//  4. For each connection, read the display name and handle the wake.
func (c *Commander) listenWake() {
	wakeListenOnce.Do(func() {
		// Step 2: open the wake socket.
		ln, err := net.Listen("unix", wakeAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[workers] listen wake: %v\n", err)
			return
		}
		// Step 3: accept connections in the background.
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					// Listener closed — stop accepting.
					return
				}
				// Step 4: handle each connection in its own goroutine.
				go func(nc net.Conn) {
					defer nc.Close()
					// Read the display name (up to 256 bytes).
					buf := make([]byte, 256)
					n, _ := nc.Read(buf)
					name := strings.TrimSpace(string(buf[:n]))
					// Process the wake message.
					c.handleWake(name)
				}(conn)
			}
		}()
	})
}

// handleWake runs on the master process: it makes sure the right workers
// exist and then signals them locally so they re-check their settings.
// A name of "*" (or empty) means "all displays".
func (c *Commander) handleWake(name string) {
	// Wake all displays when the name is "*" or empty.
	if name == "*" || name == "" {
		c.ensureAllThemeWorkers()
		signalAllWorkersLocal()
		return
	}
	// Wake a single display: start its worker if it has a library selected.
	d, ok, err := c.getDisplay(name)
	if err == nil && ok && d.Theme != 0 {
		c.ensureWorker(d)
	}
	// Signal the local worker (if it exists in this process).
	signalWorkerLocal(name)
}

// retryWorkerLock keeps retrying to become the master every few seconds.
// If the current master process dies, this process takes over: it acquires
// the lock, starts the wake listener, and starts all workers.
func (c *Commander) retryWorkerLock() {
	// Retry forever until this process becomes the master.
	for {
		// Wait before each retry.
		time.Sleep(retryEvery)
		// Check whether we already became the master (should not happen).
		wakeMu.Lock()
		master := isWorkerMaster
		wakeMu.Unlock()
		if master {
			return
		}
		// Try to take the lock from the (possibly dead) master.
		if tryAcquireWorkerLock() {
			// We are the new master: listen for wakes and start workers.
			c.listenWake()
			c.ensureAllThemeWorkers()
			return
		}
	}
}
