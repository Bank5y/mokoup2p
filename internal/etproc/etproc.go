// Package etproc runs an unmodified easytier-core binary as a supervised
// child process. It is shared by the client engine (client/engine/easytier)
// and the relay wrapper (cmd/mokoup2p-relay).
package etproc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Version is the pinned EasyTier release. Upgrading it means re-running the
// seam-3 smoke test (deploy/smoketest).
const Version = "2.6.4"

// Spec describes the child process.
type Spec struct {
	// Path is the easytier-core binary.
	Path string
	Args []string
	// Env is added to the daemon's environment. Secrets go here rather
	// than in Args: /proc/<pid>/cmdline is readable by every local user,
	// /proc/<pid>/environ only by the owner.
	Env []string
	// Name prefixes the child's log lines.
	Name string
	// OnStart, if set, is called with the PID of every (re)started child,
	// e.g. to write a PID file for leftover cleanup.
	OnStart func(pid int)
}

// Process is a running easytier-core. If the child exits on its own it is
// started again after a backoff, until Stop is called.
type Process struct {
	spec Spec

	mu       sync.Mutex
	cmd      *exec.Cmd
	exited   chan struct{} // closed when the current child exits
	stopping bool
	stop     chan struct{} // closed by Stop
	stopOnce sync.Once
	done     chan struct{} // closed when the supervisor loop ends
}

// Start launches the child. It fails only if the binary cannot be executed.
func Start(spec Spec) (*Process, error) {
	p := &Process{spec: spec, stop: make(chan struct{}), done: make(chan struct{})}
	if err := p.launch(); err != nil {
		return nil, err
	}
	go p.supervise()
	return p, nil
}

func (p *Process) launch() error {
	cmd := exec.Command(p.spec.Path, p.spec.Args...)
	cmd.Env = append(os.Environ(), p.spec.Env...)
	cmd.SysProcAttr = sysProcAttr()
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", p.spec.Path, err)
	}
	exited := make(chan struct{})
	p.mu.Lock()
	p.cmd = cmd
	p.exited = exited
	p.mu.Unlock()
	if p.spec.OnStart != nil {
		p.spec.OnStart(cmd.Process.Pid)
	}
	go p.copyLog(out)
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		stopping := p.stopping
		p.mu.Unlock()
		if !stopping {
			log.Printf("%s: exited unexpectedly: %v", p.spec.Name, err)
		}
		close(exited)
	}()
	return nil
}

func (p *Process) copyLog(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		log.Printf("%s: %s", p.spec.Name, sc.Text())
	}
}

// supervise restarts the child whenever it exits while not stopping.
func (p *Process) supervise() {
	defer close(p.done)
	backoff := time.Second
	for {
		p.mu.Lock()
		exited := p.exited
		p.mu.Unlock()
		started := time.Now()
		select {
		case <-exited:
		case <-p.stop:
			return
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second
		}
		for {
			select {
			case <-time.After(backoff):
			case <-p.stop:
				return
			}
			backoff = min(backoff*2, 30*time.Second)
			if err := p.launch(); err != nil {
				log.Printf("%s: restart: %v", p.spec.Name, err)
				continue
			}
			break
		}
	}
}

// Exited reports whether the current child has exited (it may be
// restarted later).
func (p *Process) Exited() bool {
	p.mu.Lock()
	exited := p.exited
	p.mu.Unlock()
	select {
	case <-exited:
		return true
	default:
		return false
	}
}

// Stop terminates the child and its process group: SIGTERM first, SIGKILL
// if it has not exited when ctx is done or after 5 seconds. Stop waits for
// the child to be gone and is safe to call more than once.
func (p *Process) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopping = true
	p.mu.Unlock()
	p.stopOnce.Do(func() { close(p.stop) })
	<-p.done // no restart can be in flight after this

	p.mu.Lock()
	cmd, exited := p.cmd, p.exited
	p.mu.Unlock()

	select {
	case <-exited:
		return nil
	default:
	}
	_ = terminate(cmd.Process.Pid)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-exited:
		return nil
	case <-ctx.Done():
	}
	_ = kill(cmd.Process.Pid)
	select {
	case <-exited:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New(p.spec.Name + ": child did not exit after SIGKILL")
	}
}
