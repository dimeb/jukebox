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
	clones    []*exec.Cmd
	cmd       string
	rcdURL    string
	rcdArgs   []string
	mountArgs []string
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
		mountArgs: []string{
			`mount`,
		},
	}
)

func (r *Rclone) start() {
	// Start rclone rcd.
	go func() {
		cmd, err := r.command(`rcd`)
		if err != nil {
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
			if err := scanner.Err(); err != nil {
				logger.queue <- fmt.Sprint(err)
			}
		}()
	}()
}

func (r *Rclone) command(t string, args ...string) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	commonArgs := []string{
		`--config`,
		`rclone.config`,
	}
	if t == `rcd` {
		cmd = exec.Command(r.cmd, append(append(r.rcdArgs, args...), commonArgs...)...)
	} else {
		cmd = exec.Command(r.cmd, append(append(r.mountArgs, args...), commonArgs...)...)
	}
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Pdeathsig: syscall.SIGKILL,
		Pdeathsig: syscall.SIGTERM,
	}

	if err := cmd.Start(); err != nil {
		logger.queue <- fmt.Sprint(err)
		return nil, err
	}

	return cmd, nil
}
