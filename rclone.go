package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Rclone structure.
type Rclone struct {
	clones     []*exec.Cmd
	cmd        string
	rcdURL     string
	rcdArgs    []string
	listArgs   []string
	mountArgs  []string
	commonArgs []string
	remotes    []string
}

var (
	rclone = Rclone{
		cmd: `rclone`,
		rcdArgs: []string{
			`rcd`,
			`--rc-web-gui`,
			`--rc-web-gui-no-open-browser`,
			`--rc-addr="127.0.0.1:11000"`,
		},
		listArgs: []string{
			`listremotes`,
		},
		mountArgs: []string{
			`mount`,
		},
		commonArgs: []string{
			`--config`,
			`rclone.config`,
		},
	}
)

func (r *Rclone) start() {
	// Start rclone rcd.
	go func() {
		var err error

		defer func() {
			if err != nil {
				logger.queue <- fmt.Sprint(err)
			}
		}()

		cmd := exec.Command(r.cmd, append(r.rcdArgs, r.commonArgs...)...)
		cmd.Env = os.Environ()
		cmd.SysProcAttr = &syscall.SysProcAttr{
			// Pdeathsig: syscall.SIGKILL,
			Pdeathsig: syscall.SIGTERM,
		}

		if err = cmd.Start(); err != nil {
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return
		}
		go func() {
			s := ``
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				s = scanner.Text()
				if strings.Contains(s, `Web GUI is not automatically opening browser.`) {
					a := strings.Split(s, ` `)
					if len(a) > 2 {
						r.rcdURL = a[len(a)-2]
					}
				}
			}
			err = scanner.Err()
		}()
	}()

	// Get remotes.
	b, err := exec.Command(r.cmd, append(r.listArgs, r.commonArgs...)...).Output()
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		r.remotes = strings.Split(string(b), "\n")
	}
}
