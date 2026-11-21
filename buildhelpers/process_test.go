package buildhelpers

import (
	"context"
	"testing"
	"time"
)

func TestSubprocessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd, stop := CommandContext(ctx, time.Minute, "sh", "-c", "sleep 60 & wait")
	defer stop()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled process succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("process group did not terminate")
	}
}
func TestSubprocessDeadline(t *testing.T) {
	cmd, cancel := CommandContext(context.Background(), 20*time.Millisecond, "sleep", "60")
	defer cancel()
	if err := cmd.Run(); err == nil {
		t.Fatal("deadline was ignored")
	}
}
