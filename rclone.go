package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Rclone structure.
type Rclone struct {
	cmd              string
	rcdURL           string
	playingStopped   chan bool
	mounts           map[string]*exec.Cmd
	checkMountPeriod time.Duration
}

var (
	rclone = Rclone{
		cmd:              `rclone`,
		playingStopped:   make(chan bool),
		mounts:           make(map[string]*exec.Cmd),
		checkMountPeriod: 30 * time.Second,
	}
)

func (r *Rclone) start() {
	// Start/restart rcd.
	go func() {
		for {
			logger.queue <- fmt.Sprint(`starting rclone web config ...`)
			rcd := exec.Command(
				r.cmd,
				`rcd`,
				`--rc-web-gui`,
				`--rc-web-gui-no-open-browser`,
				`--rc-addr="127.0.0.1:11000"`,
			)
			rcd.Env = os.Environ()
			rcd.SysProcAttr = &syscall.SysProcAttr{
				// Pdeathsig: syscall.SIGKILL,
				Pdeathsig: syscall.SIGTERM,
			}
			err := rcd.Start()
			if err != nil {
				logger.queue <- fmt.Sprint(err)
			} else {
				stderr, err := rcd.StderrPipe()
				if err != nil {
					logger.queue <- fmt.Sprint(err)
				} else {
					b := false
					scanner := bufio.NewScanner(stderr)
					for scanner.Scan() {
						if b {
							break
						}
						s := scanner.Text()
						if strings.Contains(s, `Web GUI is not automatically opening browser.`) {
							a := strings.Split(s, ` `)
							if len(a) > 2 {
								r.rcdURL = a[len(a)-2]
							}
							b = true
						}
					}
					if err = scanner.Err(); err != nil {
						logger.queue <- fmt.Sprint(err)
					}
					r.rcdURL = ``
					stderr.Close()
				}
			}
			rcd.Process.Kill()
			time.Sleep(5 * time.Second)
		}
	}()

	unmounted := make(chan bool)

	// Mount remotes.
	go r.mount(unmounted)

	// Unmount everything unconditionally.
	dir, err := os.Open(lists.rootDir)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		defer dir.Close()
		files, err := dir.Readdir(-1)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
		for _, file := range files {
			p := lists.rootDir + file.Name()
			if file.IsDir() {
				err = r.unmount(p)
				if err != nil {
					logger.queue <- fmt.Sprint(err)
				}
			}
			err = os.Remove(p)
			if err != nil {
				logger.queue <- fmt.Sprint(err)
			}
		}
	}
	unmounted <- true

	r.checkLists()
}

func (r *Rclone) mount(c chan bool) {
	<-c
	for _, rem := range r.getRemotes() {
		remote := rem
		go func() {
			var err error

			defer func() {
				if err != nil {
					logger.queue <- fmt.Sprint(err)
				}
			}()

			mountDir := lists.rootDir + remote
			logger.queue <- fmt.Sprintf(`mounting rclone remote %s to %s ...`, remote, mountDir)
			if err = os.MkdirAll(mountDir, 0755); err != nil {
				return
			}
			cmd := exec.Command(
				r.cmd,
				`mount`,
				remote,
				mountDir,
				`--read-only`,
			)
			cmd.Env = os.Environ()
			cmd.SysProcAttr = &syscall.SysProcAttr{
				// Pdeathsig: syscall.SIGKILL,
				Pdeathsig: syscall.SIGTERM,
			}
			if err = cmd.Run(); err != nil {
				logger.queue <- fmt.Sprint(err)
			}
			err = os.Remove(mountDir)
		}()
	}
}

func (r *Rclone) unmount(dir string) error {
	var err error

	defer func() {
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

	err = exec.Command(`fusermount`, `-u`, dir).Run()
	if err == nil {
		err = os.Remove(dir)
	}
	return err
}

func (r *Rclone) getRemotes() (remotes []string) {
	logger.queue <- fmt.Sprint(`getting rclone remotes ...`)
	b, err := exec.Command(r.cmd, `listremotes`).Output()
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		remotes = strings.Split(string(b), ":\n")
	}
	return
}

func (r *Rclone) checkRemote(remote string) bool {
	if err := exec.Command(r.cmd, `size`, remote+`:`).Run(); err != nil {
		r.unmount(lists.rootDir + remote)
		return false
	}
	return true
}

// True if remote is valid directory under lists.rootDir and is not empty.
// False in case of any error or empty.
func (r *Rclone) checkMount(remote string) bool {
	mountPoint := lists.rootDir + remote
	mount, err := os.Stat(mountPoint)
	if err != nil || !mount.IsDir() {
		return false
	}
	f, err := os.Open(mountPoint)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdir(1)
	return err == nil
}

func (r *Rclone) checkMounts() {
	for remote, cmd := range r.mounts {
		mountPoint := lists.rootDir + remote
		if r.checkMount(remote) {
			continue
		}
		cmd.Process.Kill()
		exec.Command(`/bin/fusermount`, `-u`, mountPoint).Run()
		os.Remove(mountPoint)
	}
}

func (r *Rclone) checkLists() {
}
