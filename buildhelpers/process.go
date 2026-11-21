package buildhelpers

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// CommandContext bounds a subprocess and forwards termination to its process
// group, including shell/native-tool descendants. Always defer the cancel func.
func CommandContext(parent context.Context, timeout time.Duration, name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	signals, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithTimeout(signals, timeout)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd, func() { cancel(); stop() }
}
