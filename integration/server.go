package integration

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ServerConfig contains all the args and data required to launch a poet server
// instance  and connect to it via rpc client.
type ServerConfig struct {
	rpcListen string
	dataDir   string
	exe       string

	Genesis          time.Time
	EpochDuration    time.Duration
	PhaseShift       time.Duration
	CycleGap         time.Duration
	DebugLog         bool
	Reset            bool
	RESTListen       string
	GatewayAddresses []string
	GtwConnTimeout   time.Duration
}

// DefaultConfig returns a newConfig with all default values.
func DefaultConfig() (*ServerConfig, error) {
	baseDir, err := baseDir()
	if err != nil {
		return nil, err
	}

	poetPath, err := poetExecutablePath(baseDir)
	if err != nil {
		return nil, err
	}

	cfg := &ServerConfig{
		rpcListen:      "127.0.0.1:18550",
		RESTListen:     "127.0.0.1:18551",
		dataDir:        filepath.Join(baseDir, "data"),
		exe:            poetPath,
		EpochDuration:  2 * time.Second,
		PhaseShift:     time.Second / 2,
		CycleGap:       time.Second / 4,
		GtwConnTimeout: time.Second * 10,
	}

	return cfg, nil
}

// genArgs generates a slice of command line arguments from ServerConfig instance.
func (cfg *ServerConfig) genArgs() []string {
	var args []string

	args = append(args, fmt.Sprintf("--datadir=%v", cfg.dataDir))
	args = append(args, fmt.Sprintf("--rpclisten=%v", cfg.rpcListen))
	args = append(args, fmt.Sprintf("--restlisten=%v", cfg.RESTListen))
	args = append(args, fmt.Sprintf("--genesis-time=%s", cfg.Genesis.Format(time.RFC3339)))
	args = append(args, fmt.Sprintf("--epoch-duration=%s", cfg.EpochDuration))
	args = append(args, fmt.Sprintf("--phase-shift=%s", cfg.PhaseShift))
	args = append(args, fmt.Sprintf("--cycle-gap=%s", cfg.CycleGap))
	args = append(args, fmt.Sprintf("--gtw-connection-timeout=%s", cfg.GtwConnTimeout.String()))

	for _, address := range cfg.GatewayAddresses {
		args = append(args, fmt.Sprintf("--gateway=%s", address))
	}

	if cfg.DebugLog {
		args = append(args, "--debuglog")
	}
	if cfg.Reset {
		args = append(args, "--reset")
	}

	return args
}

// server houses the necessary state required to configure, launch,
// and manage poet server process.
type server struct {
	cfg *ServerConfig
	cmd *exec.Cmd

	// processExit is a channel that's closed once it's detected that the
	// process this instance is bound to has exited.
	processExit chan struct{}

	quit chan struct{}

	errChan chan error

	stdout io.Reader
	stderr io.Reader
}

// newNode creates a new poet server instance according to the passed cfg.
func newServer(cfg *ServerConfig) (*server, error) {
	return &server{
		cfg:     cfg,
		errChan: make(chan error, 1),
	}, nil
}

// start launches a new running process of poet server.
func (s *server) start() error {
	s.quit = make(chan struct{})

	args := s.cfg.genArgs()
	s.cmd = exec.Command(s.cfg.exe, args...)

	// Get stderr and stdout pipes in case the caller wants to read them.
	// We also save a copy of stderr output here, and check it below.
	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to capture server stderr: %s", err)
	}
	var stderrBuf bytes.Buffer
	s.stderr = io.TeeReader(stderr, &stderrBuf)

	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to capture server stdout: %s", err)
	}

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %s", err)
	}

	// Launch a new goroutine which that bubbles up any potential fatal
	// process errors to errChan.
	s.processExit = make(chan struct{})
	go func() {
		err := s.cmd.Wait()
		if err != nil {
			// Don't propagate 'signal: killed' error,
			// since it's an expected behavior.
			if !strings.Contains(err.Error(), "signal: killed") && stderrBuf.Len() > 0 {
				// make sure all of the input to the teereader was consumed so we can read it here.
				// ignore output and error here, we just need to make sure it was all consumed.
				_, _ = io.ReadAll(s.stderr)
				select {
				case s.errChan <- fmt.Errorf("%v | %v", err, stderrBuf.String()):
					// we successfully sent the error to the channel
				case <-s.quit:
					// we were told to quit and no one is listening for the error so don't send the error
				}
			}
		}

		// Signal any onlookers that this process has exited.
		close(s.processExit)
	}()

	return nil
}

// shutdown terminates the running poet server process, and cleans up
// all files/directories created by it.
func (s *server) shutdown(cleanup bool) error {
	if err := s.stop(); err != nil {
		return err
	}

	if !cleanup {
		return nil
	}

	return s.cleanup()
}

// stop kills the server running process, since it doesn't support
// RPC-driven stop functionality.
func (s *server) stop() error {
	// Do nothing if the process is not running.
	if s.processExit == nil {
		return nil
	}

	if err := s.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %v", err)
	}

	close(s.quit)

	<-s.processExit
	s.quit = nil
	s.processExit = nil
	return nil
}

// cleanup cleans up the temporary files/directories created by the server process.
func (s *server) cleanup() error {
	return os.RemoveAll(s.cfg.dataDir)
}
