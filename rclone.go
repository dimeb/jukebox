package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"
)

// Rclone structure.
type Rclone struct {
	cmd        string
	configFile string
	rcdURL     string
	rcd        *exec.Cmd
	mounts     map[string]*exec.Cmd
}

var (
	rclone = Rclone{
		cmd:        `rclone`,
		configFile: `rclone.config`,
		mounts:     make(map[string]*exec.Cmd),
	}
)

func (r *Rclone) start() {
	// Start rcd.
	go r.startRcd()

	// Mount remotes.
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
				`--config`,
				r.configFile,
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

	r.checkLists()
}

func (r *Rclone) startRcd() {
	var err error

	defer func() {
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

	logger.queue <- fmt.Sprint(`starting rclone web config ...`)
	r.rcd = exec.Command(
		r.cmd,
		`rcd`,
		`--rc-web-gui`,
		`--rc-web-gui-no-open-browser`,
		`--rc-addr="127.0.0.1:11000"`,
		`--config`,
		r.configFile,
	)
	r.rcd.Env = os.Environ()
	r.rcd.SysProcAttr = &syscall.SysProcAttr{
		// Pdeathsig: syscall.SIGKILL,
		Pdeathsig: syscall.SIGTERM,
	}

	if err = r.rcd.Start(); err != nil {
		return
	}

	stderr, err := r.rcd.StderrPipe()
	if err != nil {
		return
	}

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
	r.rcd.Process.Kill()
	time.Sleep(1 * time.Second)
}

func (r *Rclone) getRemotes() (remotes []string) {
	logger.queue <- fmt.Sprint(`getting rclone remotes ...`)
	b, err := exec.Command(r.cmd, `listremotes`, `--config`, r.configFile).Output()
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		remotes = strings.Split(string(b), ":\n")
	}
	return
}

func (r *Rclone) checkLists() {
	var err error

	defer func() {
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

	mounts := []string{}
	remotes := r.getRemotes()

	d, err := os.Open(lists.rootDir)
	if err != nil {
		return
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		return
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		mounts = append(mounts, path.Base(file.Name()))
	}
}
