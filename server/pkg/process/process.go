package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type LogCallback func(line string)

type Options struct {
	Binary  string
	Args    []string
	Env     []string
	LogFile string
	OnLine  LogCallback
}

type Process struct {
	opts   Options
	cmd    *exec.Cmd
	exited chan struct{}
	mu     sync.Mutex
}

func New(opts Options) *Process {
	return &Process{opts: opts}
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		select {
		case <-p.exited:
		default:
			return fmt.Errorf("process already running (pid %d)", p.cmd.Process.Pid)
		}
	}

	cmd := exec.Command(p.opts.Binary, p.opts.Args...)
	cmd.Env = append(os.Environ(), p.opts.Env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 自管 os.Pipe：避免 cmd.StdoutPipe 与 cmd.Wait 的关闭竞态。
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return fmt.Errorf("start %s: %w", p.opts.Binary, err)
	}
	// 父进程立即关闭写端：子进程退出后 drain 读到 EOF。
	pw.Close()

	p.cmd = cmd
	p.exited = make(chan struct{})

	go p.drain(pr)
	go p.wait(pr)

	return nil
}

func (p *Process) drain(pr *os.File) {
	defer pr.Close()

	var logFile *os.File
	if p.opts.LogFile != "" {
		if f, err := os.OpenFile(p.opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			logFile = f
			defer f.Close()
		}
	}

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if logFile != nil {
			_, _ = fmt.Fprintln(logFile, line)
		}
		if p.opts.OnLine != nil {
			p.opts.OnLine(line)
		}
	}
}

func (p *Process) wait(pr *os.File) {
	_ = p.cmd.Wait()
	// 关闭读端使 drain 收 EOF 退出（logFile 由 drain 关闭）。
	pr.Close()
	close(p.exited)
}

func (p *Process) Stop(timeout time.Duration) error {
	p.mu.Lock()
	cmd := p.cmd
	exited := p.exited
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	select {
	case <-exited:
		return nil
	default:
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-exited:
		return nil
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
		// SIGKILL 后进程组仍存活（如 root 成员对用户态信号免疫）→ 上报，由调用方兜底
		if err := syscall.Kill(-cmd.Process.Pid, 0); err == nil || errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("process group %d survived SIGKILL", cmd.Process.Pid)
		}
		return nil
	}
}

func (p *Process) Running() bool {
	p.mu.Lock()
	exited := p.exited
	p.mu.Unlock()
	if exited == nil {
		return false
	}
	select {
	case <-exited:
		return false
	default:
		return true
	}
}

func (p *Process) PID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return -1
}

func (p *Process) Exited() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exited == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return p.exited
}
