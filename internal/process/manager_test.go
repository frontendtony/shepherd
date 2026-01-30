//go:build !ci

package process

import (
	"context"
	"testing"
	"time"

	"github.com/frontendtony/shepherd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	return &config.Config{
		Version: 1,
		Stacks: map[string]config.Stack{
			"full": {
				Description: "Full stack",
				Groups:      []string{"tunnels", "services"},
			},
		},
		Groups: map[string]config.Group{
			"tunnels": {
				Description: "SSH tunnels",
				Processes:   []string{"bastion", "forward"},
			},
			"services": {
				Description: "Services",
				Processes:   []string{"service"},
			},
		},
		Processes: map[string]config.Process{
			"bastion": {
				Command: "sleep 3600",
			},
			"forward": {
				Command:   "sleep 3600",
				DependsOn: []string{"bastion"},
			},
			"service": {
				Command: "sleep 3600",
			},
		},
	}
}

func TestManager_StartSingleProcess(t *testing.T) {
	cfg := &config.Config{
		Processes: map[string]config.Process{
			"echo": {Command: "sleep 3600"},
		},
	}

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	err = pm.StartProcess("echo")
	require.NoError(t, err)

	states := pm.GetAllStates()
	require.Len(t, states, 1)
	assert.Equal(t, StatusRunning, states[0].Status)
}

func TestManager_StartWithDependency(t *testing.T) {
	cfg := testConfig()

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	err = pm.StartProcess("forward")
	require.NoError(t, err)

	// Both bastion and forward should be running.
	states := pm.GetAllStates()
	running := 0
	for _, s := range states {
		if s.Status == StatusRunning {
			running++
		}
	}
	assert.Equal(t, 2, running)
}

func TestManager_StopWithDependents(t *testing.T) {
	cfg := testConfig()

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	err = pm.StartProcess("forward")
	require.NoError(t, err)

	// Stop bastion - forward should also stop.
	err = pm.StopProcess("bastion")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	states := pm.GetAllStates()
	for _, s := range states {
		if s.Name == "bastion" || s.Name == "forward" {
			assert.Equal(t, StatusStopped, s.Status, "process %s should be stopped", s.Name)
		}
	}
}

func TestManager_StartGroup(t *testing.T) {
	cfg := testConfig()

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	err = pm.StartGroup("tunnels")
	require.NoError(t, err)

	states := pm.GetAllStates()
	for _, s := range states {
		if s.Name == "bastion" || s.Name == "forward" {
			assert.Equal(t, StatusRunning, s.Status, "process %s should be running", s.Name)
		}
	}
}

func TestManager_StopAll(t *testing.T) {
	cfg := testConfig()

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)

	err = pm.StartStack("full")
	require.NoError(t, err)

	err = pm.StopAll()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	states := pm.GetAllStates()
	for _, s := range states {
		assert.NotEqual(t, StatusRunning, s.Status, "process %s should not be running", s.Name)
	}
}

func TestManager_Resolve(t *testing.T) {
	cfg := testConfig()

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	kind, err := pm.Resolve("full")
	require.NoError(t, err)
	assert.Equal(t, "stack", kind)

	kind, err = pm.Resolve("tunnels")
	require.NoError(t, err)
	assert.Equal(t, "group", kind)

	kind, err = pm.Resolve("bastion")
	require.NoError(t, err)
	assert.Equal(t, "process", kind)

	_, err = pm.Resolve("nonexistent")
	assert.Error(t, err)
}

func TestManager_Events(t *testing.T) {
	cfg := &config.Config{
		Processes: map[string]config.Process{
			"echo": {Command: "sleep 3600"},
		},
	}

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	events := pm.Events()

	err = pm.StartProcess("echo")
	require.NoError(t, err)

	// Should receive a state change event.
	select {
	case ev := <-events:
		assert.Equal(t, "echo", ev.Name)
		assert.Equal(t, StatusRunning, ev.NewState)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestManager_RetryOnFailure(t *testing.T) {
	cfg := &config.Config{
		Processes: map[string]config.Process{
			"fail": {
				Command: "exit 1",
				Retry: config.RetryConfig{
					Enabled:           true,
					MaxAttempts:       2,
					InitialBackoff:    config.Duration(100 * time.Millisecond),
					MaxBackoff:        config.Duration(200 * time.Millisecond),
					BackoffMultiplier: 1.5,
				},
			},
		},
	}

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	events := pm.Events()

	err = pm.StartProcess("fail")
	require.NoError(t, err)

	// Wait for retries to complete.
	deadline := time.After(10 * time.Second)
	finalFailed := false
	for !finalFailed {
		select {
		case ev := <-events:
			if ev.Name == "fail" && ev.NewState == StatusFailed && ev.OldState == StatusFailed {
				finalFailed = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for final failure")
		}
	}
}

func TestManager_GetLogBuffer(t *testing.T) {
	cfg := &config.Config{
		Processes: map[string]config.Process{
			"echo": {Command: "echo hello"},
		},
	}

	pm, err := NewProcessManager(context.Background(), cfg)
	require.NoError(t, err)
	defer pm.Shutdown()

	buf := pm.GetLogBuffer("echo")
	assert.NotNil(t, buf)

	buf = pm.GetLogBuffer("nonexistent")
	assert.Nil(t, buf)
}
