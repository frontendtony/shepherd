package process

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/frontendtony/shepherd/internal/config"
	"github.com/frontendtony/shepherd/internal/logging"
)

const stopTimeout = 10 * time.Second

// ManagedProcess wraps an exec.Cmd with lifecycle management and PTY output capture.
type ManagedProcess struct {
	name   string
	config config.Process
	log    *logging.RingBuffer

	mu    sync.Mutex
	state ProcessState
	cmd   *exec.Cmd
	ptmx  *os.File // PTY master file descriptor (nil when using pipe fallback)
	done  chan struct{}
}

// NewManagedProcess creates a new managed process.
func NewManagedProcess(name string, cfg config.Process, logBuf *logging.RingBuffer) *ManagedProcess {
	return &ManagedProcess{
		name:   name,
		config: cfg,
		log:    logBuf,
		state: ProcessState{
			Name:   name,
			Status: StatusStopped,
		},
	}
}

// Start launches the process via PTY using sh -c.
// Falls back to pipe-based capture if PTY allocation fails.
func (p *ManagedProcess) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state.Status == StatusRunning {
		return fmt.Errorf("process %s is already running", p.name)
	}

	p.state.Status = StatusStarting

	cmd := exec.Command("sh", "-c", p.config.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if p.config.WorkingDir != "" {
		cmd.Dir = p.config.WorkingDir
	}

	cmd.Env = buildEnv(p.config.Env)

	// Try PTY first, fall back to pipes.
	var reader io.Reader
	var pipeWriter *io.PipeWriter

	ptmx, err := pty.Start(cmd)
	if err == nil {
		p.ptmx = ptmx
		reader = ptmx
	} else {
		// Fallback: use pipes for stdout/stderr.
		p.ptmx = nil
		var pr *io.PipeReader
		pr, pipeWriter = io.Pipe()
		cmd.Stdout = pipeWriter
		cmd.Stderr = pipeWriter
		reader = pr

		if err := cmd.Start(); err != nil {
			pipeWriter.Close()
			pr.Close()
			p.state.Status = StatusFailed
			p.state.LastError = err.Error()
			return fmt.Errorf("starting process %s: %w", p.name, err)
		}
	}

	p.cmd = cmd
	p.done = make(chan struct{})
	p.state.Status = StatusRunning
	p.state.PID = cmd.Process.Pid
	p.state.StartedAt = time.Now()
	p.state.StoppedAt = time.Time{}
	p.state.LastError = ""
	p.state.ExitCode = 0

	// Read output into log buffer.
	go p.readOutput(reader)

	// Monitor process exit.
	go p.waitForExit(pipeWriter)

	return nil
}

// Stop sends SIGTERM to the process group, then SIGKILL after timeout.
func (p *ManagedProcess) Stop() error {
	p.mu.Lock()

	if p.state.Status != StatusRunning && p.state.Status != StatusStarting {
		p.mu.Unlock()
		return nil
	}

	p.state.Status = StatusStopping
	cmd := p.cmd
	done := p.done
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Send SIGTERM to process group.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

	// Wait for exit or timeout.
	select {
	case <-done:
		return nil
	case <-time.After(stopTimeout):
		// Force kill.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return nil
	}
}

// Wait returns a channel that closes when the process exits.
func (p *ManagedProcess) Wait() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return p.done
}

// State returns a thread-safe snapshot of the current process state.
func (p *ManagedProcess) State() ProcessState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Name returns the process name.
func (p *ManagedProcess) Name() string {
	return p.name
}

// SetStatus sets the status (used by the manager for retrying/failed states).
func (p *ManagedProcess) SetStatus(status Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.Status = status
}

// SetRetryState updates retry-related fields.
func (p *ManagedProcess) SetRetryState(count int, nextRetry time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.RetryCount = count
	p.state.NextRetryAt = nextRetry
}

// SetError sets the last error message.
func (p *ManagedProcess) SetError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.LastError = err
}

// ResetRetryCount resets the retry counter.
func (p *ManagedProcess) ResetRetryCount() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.RetryCount = 0
	p.state.NextRetryAt = time.Time{}
}

func (p *ManagedProcess) readOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	for scanner.Scan() {
		p.log.Write(append(scanner.Bytes(), '\n'))
	}
}

// waitForExit waits for the process to exit and updates state.
// If pw is non-nil (pipe fallback mode), it closes the pipe writer after cmd.Wait().
func (p *ManagedProcess) waitForExit(pw *io.PipeWriter) {
	err := p.cmd.Wait()

	// Close PTY or pipe writer.
	if p.ptmx != nil {
		p.ptmx.Close()
	}
	if pw != nil {
		pw.Close()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.StoppedAt = time.Now()
	p.state.PID = 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			p.state.ExitCode = exitErr.ExitCode()
		}
		if p.state.Status == StatusStopping {
			p.state.Status = StatusStopped
		} else {
			p.state.Status = StatusFailed
			p.state.LastError = err.Error()
		}
	} else {
		p.state.ExitCode = 0
		p.state.Status = StatusStopped
	}

	close(p.done)
}

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
