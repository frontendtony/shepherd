//go:build !ci

package process

import (
	"testing"
	"time"

	"github.com/frontendtony/shepherd/internal/config"
	"github.com/frontendtony/shepherd/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProcess(command string) (*ManagedProcess, *logging.RingBuffer) {
	buf := logging.NewRingBuffer(100)
	proc := NewManagedProcess("test", config.Process{
		Command: command,
	}, buf)
	return proc, buf
}

func TestProcess_StartAndExit(t *testing.T) {
	proc, buf := newTestProcess("echo hello")

	err := proc.Start()
	require.NoError(t, err)

	state := proc.State()
	assert.Equal(t, StatusRunning, state.Status)
	assert.NotZero(t, state.PID)

	// Wait for process to exit.
	select {
	case <-proc.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}

	state = proc.State()
	assert.Equal(t, StatusStopped, state.Status)
	assert.Equal(t, 0, state.ExitCode)
	assert.NotZero(t, state.StoppedAt)

	// Check output was captured.
	time.Sleep(50 * time.Millisecond) // Small delay for PTY read goroutine.
	lines := buf.All()
	assert.NotEmpty(t, lines)
}

func TestProcess_StartAndStop(t *testing.T) {
	proc, _ := newTestProcess("sleep 3600")

	err := proc.Start()
	require.NoError(t, err)

	state := proc.State()
	assert.Equal(t, StatusRunning, state.Status)

	err = proc.Stop()
	require.NoError(t, err)

	state = proc.State()
	assert.Equal(t, StatusStopped, state.Status)
}

func TestProcess_FailedCommand(t *testing.T) {
	proc, _ := newTestProcess("exit 42")

	err := proc.Start()
	require.NoError(t, err)

	select {
	case <-proc.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}

	state := proc.State()
	assert.Equal(t, StatusFailed, state.Status)
	assert.Equal(t, 42, state.ExitCode)
	assert.NotEmpty(t, state.LastError)
}

func TestProcess_NonexistentCommand(t *testing.T) {
	proc, _ := newTestProcess("this_command_does_not_exist_12345")

	err := proc.Start()
	require.NoError(t, err) // sh -c will start, then the shell will fail

	select {
	case <-proc.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}

	state := proc.State()
	assert.Equal(t, StatusFailed, state.Status)
	assert.NotZero(t, state.ExitCode)
}

func TestProcess_DoubleStart(t *testing.T) {
	proc, _ := newTestProcess("sleep 3600")

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	err = proc.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestProcess_StopNotRunning(t *testing.T) {
	proc, _ := newTestProcess("echo hi")

	err := proc.Stop()
	assert.NoError(t, err) // Should be a no-op.
}

func TestProcess_Uptime(t *testing.T) {
	proc, _ := newTestProcess("sleep 3600")

	err := proc.Start()
	require.NoError(t, err)
	defer proc.Stop()

	time.Sleep(100 * time.Millisecond)
	state := proc.State()
	assert.True(t, state.Uptime() >= 100*time.Millisecond)
}

func TestProcess_OutputCapture(t *testing.T) {
	proc, buf := newTestProcess("echo line1 && echo line2 && echo line3")

	err := proc.Start()
	require.NoError(t, err)

	select {
	case <-proc.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}

	// Small delay for PTY read goroutine to finish.
	time.Sleep(100 * time.Millisecond)

	lines := buf.All()
	assert.GreaterOrEqual(t, len(lines), 3)
}

func TestProcess_WorkingDir(t *testing.T) {
	buf := logging.NewRingBuffer(100)
	proc := NewManagedProcess("test", config.Process{
		Command:    "pwd",
		WorkingDir: "/tmp",
	}, buf)

	err := proc.Start()
	require.NoError(t, err)

	select {
	case <-proc.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}

	time.Sleep(100 * time.Millisecond)
	lines := buf.All()
	assert.NotEmpty(t, lines)
	// On macOS /tmp -> /private/tmp.
	found := false
	for _, l := range lines {
		if contains(l, "/tmp") || contains(l, "/private/tmp") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected /tmp in output, got: %v", lines)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestProcess_EnvVars(t *testing.T) {
	buf := logging.NewRingBuffer(100)
	proc := NewManagedProcess("test", config.Process{
		Command: "echo $SHEPHERD_TEST_VAR",
		Env:     map[string]string{"SHEPHERD_TEST_VAR": "hello_from_shepherd"},
	}, buf)

	err := proc.Start()
	require.NoError(t, err)

	select {
	case <-proc.Wait():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}

	time.Sleep(100 * time.Millisecond)
	lines := buf.All()
	found := false
	for _, l := range lines {
		if containsStr(l, "hello_from_shepherd") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected env var in output, got: %v", lines)
}
