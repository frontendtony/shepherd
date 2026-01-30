package process

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/frontendtony/shepherd/internal/config"
	"github.com/frontendtony/shepherd/internal/logging"
)

const depHealthDelay = 2 * time.Second

// StateEvent is emitted when a process changes state.
type StateEvent struct {
	Name     string
	OldState Status
	NewState Status
	Error    string
}

// ProcessManager orchestrates multiple processes with dependency resolution and retry logic.
type ProcessManager struct {
	config     *config.Config
	graph      *DependencyGraph
	processes  map[string]*ManagedProcess
	logBuffers map[string]*logging.RingBuffer
	events     chan StateEvent
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewProcessManager creates a manager from the given config.
func NewProcessManager(ctx context.Context, cfg *config.Config) (*ProcessManager, error) {
	graph := NewDependencyGraph(cfg)
	if err := graph.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependency graph: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)

	pm := &ProcessManager{
		config:     cfg,
		graph:      graph,
		processes:  make(map[string]*ManagedProcess),
		logBuffers: make(map[string]*logging.RingBuffer),
		events:     make(chan StateEvent, 100),
		ctx:        childCtx,
		cancel:     cancel,
	}

	for name, proc := range cfg.Processes {
		buf := logging.NewRingBuffer(logging.DefaultBufferSize)
		pm.logBuffers[name] = buf
		pm.processes[name] = NewManagedProcess(name, proc, buf)
	}

	return pm, nil
}

// Events returns the channel for receiving state change events.
func (pm *ProcessManager) Events() <-chan StateEvent {
	return pm.events
}

// GetAllStates returns a snapshot of all process states.
func (pm *ProcessManager) GetAllStates() []ProcessState {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	states := make([]ProcessState, 0, len(pm.processes))
	for _, p := range pm.processes {
		states = append(states, p.State())
	}
	return states
}

// GetLogBuffer returns the log buffer for a specific process.
func (pm *ProcessManager) GetLogBuffer(name string) *logging.RingBuffer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.logBuffers[name]
}

// GetConfig returns the config.
func (pm *ProcessManager) GetConfig() *config.Config {
	return pm.config
}

// StartProcess starts a process and all its transitive dependencies.
func (pm *ProcessManager) StartProcess(name string) error {
	order, err := pm.graph.StartOrder([]string{name})
	if err != nil {
		return err
	}
	return pm.startInOrder(order)
}

// StopProcess stops a process and all its dependents first.
func (pm *ProcessManager) StopProcess(name string) error {
	// Find dependents that are currently running.
	dependents := pm.graph.Dependents(name)

	// Stop dependents first (they depend on this process).
	for _, dep := range dependents {
		pm.mu.RLock()
		p := pm.processes[dep]
		pm.mu.RUnlock()

		state := p.State()
		if state.Status == StatusRunning || state.Status == StatusStarting || state.Status == StatusRetrying {
			if err := pm.stopSingle(dep); err != nil {
				slog.Warn("failed to stop dependent", "process", dep, "error", err)
			}
		}
	}

	return pm.stopSingle(name)
}

// RestartProcess stops a process and its dependents, then restarts the process.
// Dependents that were failed due to this dependency are auto-restarted.
func (pm *ProcessManager) RestartProcess(name string) error {
	// Track which dependents were running or failed due to dependency.
	dependents := pm.graph.Dependents(name)
	restartDeps := make([]string, 0)

	for _, dep := range dependents {
		pm.mu.RLock()
		p := pm.processes[dep]
		pm.mu.RUnlock()

		state := p.State()
		if state.Status == StatusRunning || state.Status == StatusStarting ||
			state.Status == StatusFailed || state.Status == StatusRetrying {
			restartDeps = append(restartDeps, dep)
		}
	}

	if err := pm.StopProcess(name); err != nil {
		return fmt.Errorf("stopping %s for restart: %w", name, err)
	}

	// Start the process itself.
	if err := pm.startSingle(name); err != nil {
		return fmt.Errorf("restarting %s: %w", name, err)
	}

	// Auto-restart dependents.
	for _, dep := range restartDeps {
		pm.mu.RLock()
		p := pm.processes[dep]
		pm.mu.RUnlock()
		p.ResetRetryCount()

		if err := pm.startSingle(dep); err != nil {
			slog.Warn("failed to restart dependent", "process", dep, "error", err)
		}
	}

	return nil
}

// StartGroup starts all processes in the named group.
func (pm *ProcessManager) StartGroup(groupName string) error {
	group, ok := pm.config.Groups[groupName]
	if !ok {
		return fmt.Errorf("unknown group: %s", groupName)
	}

	// Collect all processes and their dependencies.
	var allTargets []string
	allTargets = append(allTargets, group.Processes...)

	order, err := pm.graph.StartOrder(allTargets)
	if err != nil {
		return err
	}
	return pm.startInOrder(order)
}

// StartStack starts all groups in the named stack.
func (pm *ProcessManager) StartStack(stackName string) error {
	stack, ok := pm.config.Stacks[stackName]
	if !ok {
		return fmt.Errorf("unknown stack: %s", stackName)
	}

	var allTargets []string
	for _, groupName := range stack.Groups {
		group, ok := pm.config.Groups[groupName]
		if !ok {
			return fmt.Errorf("stack %s references unknown group %s", stackName, groupName)
		}
		allTargets = append(allTargets, group.Processes...)
	}

	order, err := pm.graph.StartOrder(allTargets)
	if err != nil {
		return err
	}
	return pm.startInOrder(order)
}

// Resolve resolves a name to its type (stack, group, or process).
func (pm *ProcessManager) Resolve(name string) (kind string, err error) {
	if _, ok := pm.config.Stacks[name]; ok {
		return "stack", nil
	}
	if _, ok := pm.config.Groups[name]; ok {
		return "group", nil
	}
	if _, ok := pm.config.Processes[name]; ok {
		return "process", nil
	}
	return "", fmt.Errorf("unknown name: %s (not a stack, group, or process)", name)
}

// StartByName resolves a name and starts the corresponding stack/group/process.
func (pm *ProcessManager) StartByName(name string) error {
	kind, err := pm.Resolve(name)
	if err != nil {
		return err
	}
	switch kind {
	case "stack":
		return pm.StartStack(name)
	case "group":
		return pm.StartGroup(name)
	case "process":
		return pm.StartProcess(name)
	}
	return nil
}

// StopAll stops all running processes in reverse dependency order.
func (pm *ProcessManager) StopAll() error {
	pm.mu.RLock()
	var running []string
	for name, p := range pm.processes {
		state := p.State()
		if state.Status == StatusRunning || state.Status == StatusStarting ||
			state.Status == StatusRetrying || state.Status == StatusStopping {
			running = append(running, name)
		}
	}
	pm.mu.RUnlock()

	if len(running) == 0 {
		return nil
	}

	order, err := pm.graph.StopOrder(running)
	if err != nil {
		// If graph fails, just stop everything.
		for _, name := range running {
			pm.stopSingle(name)
		}
		return nil
	}

	for _, name := range order {
		pm.mu.RLock()
		p := pm.processes[name]
		pm.mu.RUnlock()

		state := p.State()
		if state.Status == StatusRunning || state.Status == StatusStarting ||
			state.Status == StatusRetrying {
			if err := pm.stopSingle(name); err != nil {
				slog.Warn("failed to stop process during StopAll", "process", name, "error", err)
			}
		}
	}

	return nil
}

// Shutdown cancels the context and stops all processes.
func (pm *ProcessManager) Shutdown() {
	pm.cancel()
	pm.StopAll()
}

// startInOrder starts processes sequentially in dependency order, skipping already-running ones.
func (pm *ProcessManager) startInOrder(order []string) error {
	for _, name := range order {
		select {
		case <-pm.ctx.Done():
			return pm.ctx.Err()
		default:
		}

		pm.mu.RLock()
		p := pm.processes[name]
		pm.mu.RUnlock()

		state := p.State()

		// Skip already running.
		if state.Status == StatusRunning {
			continue
		}

		// Check if any dependency has permanently failed.
		deps := pm.graph.Dependencies(name)
		for _, dep := range deps {
			pm.mu.RLock()
			dp := pm.processes[dep]
			pm.mu.RUnlock()

			depState := dp.State()
			if depState.Status == StatusFailed {
				errMsg := fmt.Sprintf("dependency %s failed", dep)
				p.SetStatus(StatusFailed)
				p.SetError(errMsg)
				pm.emitEvent(name, state.Status, StatusFailed, errMsg)
				return fmt.Errorf("cannot start %s: %s", name, errMsg)
			}
		}

		// Wait for direct dependencies to be running and healthy.
		procCfg := pm.config.Processes[name]
		for _, dep := range procCfg.DependsOn {
			if err := pm.waitForHealthy(dep); err != nil {
				return fmt.Errorf("waiting for dependency %s: %w", dep, err)
			}
		}

		if err := pm.startSingle(name); err != nil {
			return err
		}
	}
	return nil
}

// startSingle starts a single process and sets up monitoring.
func (pm *ProcessManager) startSingle(name string) error {
	pm.mu.RLock()
	p := pm.processes[name]
	pm.mu.RUnlock()

	oldStatus := p.State().Status
	if err := p.Start(); err != nil {
		pm.emitEvent(name, oldStatus, StatusFailed, err.Error())
		return err
	}
	pm.emitEvent(name, oldStatus, StatusRunning, "")

	// Monitor this process for exit.
	go pm.monitor(name)

	return nil
}

// stopSingle stops a single process, cancelling any pending retry.
func (pm *ProcessManager) stopSingle(name string) error {
	pm.mu.RLock()
	p := pm.processes[name]
	pm.mu.RUnlock()

	oldStatus := p.State().Status

	// Cancel any pending retry by setting to stopping.
	if oldStatus == StatusRetrying {
		p.SetStatus(StatusStopped)
		pm.emitEvent(name, oldStatus, StatusStopped, "")
		return nil
	}

	if err := p.Stop(); err != nil {
		return err
	}
	pm.emitEvent(name, oldStatus, StatusStopped, "")
	return nil
}

// monitor watches a process and handles retries on failure.
func (pm *ProcessManager) monitor(name string) {
	pm.mu.RLock()
	p := pm.processes[name]
	pm.mu.RUnlock()

	<-p.Wait()

	state := p.State()

	// If process was intentionally stopped, nothing to do.
	if state.Status == StatusStopped {
		return
	}

	// Process failed - check retry logic.
	procCfg := pm.config.Processes[name]
	retryCount := state.RetryCount

	if shouldRetry(retryCount, procCfg.Retry) {
		backoff := nextBackoff(retryCount, procCfg.Retry)
		nextRetry := time.Now().Add(backoff)
		p.SetStatus(StatusRetrying)
		p.SetRetryState(retryCount+1, nextRetry)
		pm.emitEvent(name, StatusFailed, StatusRetrying, "")

		slog.Info("scheduling retry", "process", name, "attempt", retryCount+1, "backoff", backoff)

		// Wait for backoff period.
		select {
		case <-pm.ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Check if we were stopped during the backoff.
		currentState := p.State()
		if currentState.Status != StatusRetrying {
			return
		}

		if err := pm.startSingle(name); err != nil {
			slog.Error("retry failed", "process", name, "error", err)
			// startSingle will emit events and the next monitor call will handle further retries.
		}
	} else {
		// Max retries exhausted - cascade failure.
		p.SetStatus(StatusFailed)
		pm.emitEvent(name, StatusFailed, StatusFailed, fmt.Sprintf("max retries exhausted (exit code %d)", state.ExitCode))

		pm.cascadeFailure(name)
	}
}

// cascadeFailure marks all dependents of a failed process as failed.
func (pm *ProcessManager) cascadeFailure(name string) {
	dependents := pm.graph.Dependents(name)
	for _, dep := range dependents {
		pm.mu.RLock()
		p := pm.processes[dep]
		pm.mu.RUnlock()

		state := p.State()
		if state.Status == StatusRunning || state.Status == StatusStarting || state.Status == StatusRetrying {
			// Stop the dependent first.
			pm.stopSingle(dep)
		}

		errMsg := fmt.Sprintf("dependency %s failed", name)
		oldStatus := p.State().Status
		p.SetStatus(StatusFailed)
		p.SetError(errMsg)
		pm.emitEvent(dep, oldStatus, StatusFailed, errMsg)
	}
}

// waitForHealthy blocks until the named process has been running for depHealthDelay.
func (pm *ProcessManager) waitForHealthy(name string) error {
	timeout := 60 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-pm.ctx.Done():
			return pm.ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s to become healthy", name)
		}

		pm.mu.RLock()
		p := pm.processes[name]
		pm.mu.RUnlock()

		state := p.State()
		if state.Status == StatusFailed {
			return fmt.Errorf("dependency %s is in failed state", name)
		}
		if state.Status == StatusRunning && time.Since(state.StartedAt) >= depHealthDelay {
			return nil
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func (pm *ProcessManager) emitEvent(name string, oldState, newState Status, errMsg string) {
	select {
	case pm.events <- StateEvent{
		Name:     name,
		OldState: oldState,
		NewState: newState,
		Error:    errMsg,
	}:
	default:
		// Drop event if channel is full (shouldn't happen with buffer of 100).
		slog.Warn("event channel full, dropping event", "process", name)
	}
}
