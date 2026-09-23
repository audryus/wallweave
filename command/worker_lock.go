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

// Com 2 monitores o shell instancia 2 widgets → 2 processos wallweave.
// Só um pode rodar os workers de wallpaper; o outro acorda o master via socket.
const (
	lockPath   = "wallweave.workers.lock"
	wakeAddr   = "\x00wallweave.wake" // abstract socket (auto-limpa no exit)
	retryEvery = 5 * time.Second
)

var (
	lockFile       *os.File
	isWorkerMaster bool
	wakeListenOnce sync.Once
	wakeMu         sync.Mutex
)

func tryAcquireWorkerLock() bool {
	wakeMu.Lock()
	defer wakeMu.Unlock()
	if isWorkerMaster {
		return true
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}
	lockFile = f
	isWorkerMaster = true
	return true
}

func releaseWorkerLock() {
	wakeMu.Lock()
	defer wakeMu.Unlock()
	if lockFile != nil {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		lockFile = nil
	}
	isWorkerMaster = false
}

// sendWake notifica o master via abstract unix socket.
func sendWake(name string) {
	conn, err := net.Dial("unix", wakeAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = conn.Write([]byte(name + "\n"))
}

// listenWake aceita wakes de outros processos (só o master chama).
func (c *Commander) listenWake() {
	wakeListenOnce.Do(func() {
		ln, err := net.Listen("unix", wakeAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[workers] listen wake: %v\n", err)
			return
		}
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				go func(nc net.Conn) {
					defer nc.Close()
					buf := make([]byte, 256)
					n, _ := nc.Read(buf)
					name := strings.TrimSpace(string(buf[:n]))
					c.handleWake(name)
				}(conn)
			}
		}()
	})
}

// handleWake roda no master: ensure + sinaliza canal local.
func (c *Commander) handleWake(name string) {
	if name == "*" || name == "" {
		c.ensureAllThemeWorkers()
		signalAllWorkersLocal()
		return
	}
	d, ok, err := c.getDisplay(name)
	if err == nil && ok && d.Theme != 0 {
		c.ensureWorker(d)
	}
	signalWorkerLocal(name)
}

// retryWorkerLock: se o master morrer, este processo assume.
func (c *Commander) retryWorkerLock() {
	for {
		time.Sleep(retryEvery)
		wakeMu.Lock()
		master := isWorkerMaster
		wakeMu.Unlock()
		if master {
			return
		}
		if tryAcquireWorkerLock() {
			c.listenWake()
			c.ensureAllThemeWorkers()
			return
		}
	}
}
