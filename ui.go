// Package sand is for creating interpreters.
//
// This package implements a concurrent model for an interpreter. Which views
// an interpreter as two separate components, a User Interface (UI) and a Command
// Processor (Engine). The UI is provided for you, whereas, Engine implementations
// must be provided.
//
package sand

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
)

// errNoEngine represents an interpreter trying to be run without a backing engine.
var errNoEngine = errors.New("sand: engine must be non-null")

// IsRecoverable guesses if the provided error is considered
// recoverable from. In the sense that the main function can keep
// running and not log.Fatal or retry or something of that nature.
// It will default to true for any unknown error, so the caller
// still needs to do their own error handling of the root error.
//
// An example of a recoverable error is an io.EOF if a
// bytes.Buffer/Reader is used as the input Reader for a UI. This
// error is obviously recoverable to a human but in this case but
// a computer has no way of determining that itself.
//
// Recoverable Errors:
//		- err == nil
//		- context.Cancelled
// 		- context.DeadlineExceeded
//		- newLineErr (an internal error, which isn't really important)
//
func IsRecoverable(err error) (root error, ok bool) {
	if err == nil {
		return nil, true
	}

	root = errors.Cause(err)

	// Check Sentinel errors
	if root == context.DeadlineExceeded || root == context.Canceled {
		return root, true
	}

	// Check error types
errTypes:
	switch v := root.(type) {
	case net.Error:
	case runtime.Error:
	case newLineErr:
		root = v.werr
		goto errTypes
	default:
		return root, true
	}

	return
}

// SignalHandler is a type that transforms incoming interrupt
// signals the UI has received.
//
type SignalHandler func(os.Signal) os.Signal

// Option represents setting an option for the interpreter UI.
//
type Option func(*UI)

// WithPrefix specifies the prefix
//
func WithPrefix(prefix string) Option {
	return func(ui *UI) {
		ui.prefix = []byte(prefix)
	}
}

// WithIO specifies the Reader and Writer to use for IO.
//
func WithIO(in io.Reader, out io.Writer) Option {
	return func(ui *UI) {
		ui.i = in
		ui.o = out
	}
}

// WithSignalHandlers specifies user provided signal handlers to register.
//
func WithSignalHandlers(handlers map[os.Signal]SignalHandler) Option {
	return func(ui *UI) {
		ui.sigHandlers = handlers
	}
}

// UI represents the user interface for the interpreter.
// UI listens for all signals and handles them as graceful
// as possible. If signal handlers are provided then the
// handling of the Interrupt and Kill signal can be overwritten.
// By default, UI will shutdown on Interrupt and Kill signals.
//
type UI struct {
	// I/O shit
	i           io.Reader
	o           io.Writer
	prefix      []byte
	sigHandlers map[os.Signal]SignalHandler

	ctx context.Context // This is reset for every Run call
}

// SetPrefix sets the interpreters line prefix
//
func (ui *UI) SetPrefix(prefix string) {
	ui.prefix = []byte(prefix)
}

// SetIO sets the interpreters I/O.
//
func (ui *UI) SetIO(in io.Reader, out io.Writer) {
	ui.i = in
	ui.o = out
}

// Run creates a UI and associates the provided Engine to it.
// It then starts the UI.
//
func Run(ctx context.Context, eng Engine, opts ...Option) error {
	ui := new(UI)
	return ui.Run(ctx, eng, opts...)
}

// minRead
const minRead = 512

// newLineErr is used for internal use when checking recoverable errors
type newLineErr struct {
	werr error
}

func (e newLineErr) Error() string {
	return fmt.Sprintf("sand: encountered error when writing newline, %s", e.werr)
}

// Run starts the user interface with the provided sources
// for input and output of the interpreter and engine.
// The prefix will be printed before every line.
//
func (ui *UI) Run(ctx context.Context, eng Engine, opts ...Option) (err error) {
	// Make sure engine is set
	if eng == nil {
		panic(errNoEngine)
	}

	// Catch any panics
	defer func() {
		if r := recover(); r != nil {
			rerr, ok := r.(error)
			if !ok {
				return
			}

			err = errors.Wrap(rerr, "sand: recovered from panic")
		}
	}()

	// Set options
	for _, opt := range opts {
		opt(ui)
	}

	// Check if context is nil
	var cancel context.CancelFunc
	if ctx == nil {
		ctx, cancel = context.WithCancel(context.Background())
	}

	ui.ctx = ctx
	if cancel == nil {
		ui.ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Set up channels
	reqCh := make(chan execReq)
	sigs := make(chan os.Signal, 1)
	defer close(reqCh)

	// Start engine and signal monitoring
	go ui.monitorSys(ui.ctx, cancel, sigs)
	ui.startEngine(ctx, eng, reqCh)

	// Now, begin reading lines from input.
	defer func() {
		if err == nil || err == io.EOF {
			_, err = ui.o.Write([]byte("\n"))
			if err != nil {
				err = newLineErr{werr: err}
			}
			return
		}
	}()

	var n int
	for {
		// Write prefix
		_, err = ui.Write(nil)
		if err != nil {
			err = errors.Wrap(err, "sand: encountered error while writing prefix")
			return
		}

		// Read line
		b := make([]byte, minRead)
		n, err = ui.Read(b)
		if err != nil && err != io.EOF || n == 0 {
			return
		}

		// Truncate nil bytes
		idx := bytes.IndexByte(b, 0)
		if idx != -1 {
			b = b[:idx]
		}

		// Execute line
		status := ui.exec(ui.ctx, string(b), reqCh)
		if status != 0 {
			return
		}

		// Check if we hit EOF on previous read
		if err == io.EOF {
			return
		}
	}
}

var engines = struct {
	sync.Mutex
	engs map[Engine]chan chan execReq
}{
	engs: make(map[Engine]chan chan execReq),
}

// startEngine starts the provided engine and uses it
// to execute commands.
//
func (ui *UI) startEngine(ctx context.Context, eng Engine, uiReqCh chan execReq) {
	engines.Lock()
	reqCh, exists := engines.engs[eng]
	if !exists {
		reqCh = make(chan chan execReq)
		engines.engs[eng] = reqCh
		go runEngine(ctx, eng, reqCh)
	}
	engines.Unlock()

	reqCh <- uiReqCh
}

// monitorSys monitors syscalls from the OS
//
func (ui *UI) monitorSys(ctx context.Context, cancel context.CancelFunc, sigCh chan os.Signal) {
	signal.Notify(sigCh)
	defer close(sigCh)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ctx.Done():
		case sig := <-sigCh:
			handler, exists := ui.sigHandlers[sig]
			if exists {
				sig = handler(sig)
			}
			if sig == os.Kill || sig == os.Interrupt {
				cancel()
			}
		}
	}
}

// ioResp represents the response parameters from either a Read or Write call.
type ioResp struct {
	n   int
	err error
}

// readAsync wraps a Read call and sends the result to the given channel
//
func (ui *UI) readAsync(b []byte, readCh chan ioResp) {
	var resp ioResp
	resp.n, resp.err = ui.i.Read(b)
	select {
	case <-ui.ctx.Done():
	case readCh <- resp:
	}
	close(readCh)
}

// Read reads from the underlying input Reader.
// This is a blocking call and handles monitoring
// the current context. Thus, callers should handle
// context errors appropriately. See examples for
// such handling.
//
func (ui *UI) Read(b []byte) (n int, err error) {
	readCh := make(chan ioResp, 1)

	go ui.readAsync(b, readCh)

	select {
	case <-ui.ctx.Done():
		err = ui.ctx.Err()
		return
	case resp := <-readCh:
		n = resp.n
		err = resp.err
	}
	return
}

// writeAsync wraps a Write call and send the result to the given channel
//
func (ui *UI) writeAsync(b []byte, writeCh chan ioResp) {
	var resp ioResp
	resp.n, resp.err = ui.o.Write(b)
	select {
	case <-ui.ctx.Done():
	case writeCh <- resp:
	}
	close(writeCh)
}

// Write writes the provided bytes to the UIs underlying
// output along with the prefix characters.
//
// In order to avoid data races due to the UI prefix, any
// changes to the prefix must be done in a serial pair of
// SetPrefix and Write calls. This means multiple goroutines
// cannot call SetPrefix + Write, simultaneously. See example
// "tictactoe" for a demonstration of changing the prefix.
//
func (ui *UI) Write(b []byte) (n int, err error) {
	prefix := ui.prefix
	if prefix == nil && b == nil { // skips writing empty prefix call in Run call
		return
	}

	writeCh := make(chan ioResp, 1)
	go ui.writeAsync(append(prefix, b...), writeCh)

	select {
	case <-ui.ctx.Done():
		err = ui.ctx.Err()
		return
	case resp := <-writeCh:
		n = resp.n
		err = resp.err
	}
	return
}
