// Package sand is for creating interpreters.
//
// This package implements a concurrent model for an interpreter. Which views
// an interpreter as two separate components, a User Interface (UI) and a Command
// Processor (Engine). The UI is provided for you, whereas, Engine implementations
// must be provided.
//
package sand

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
)

// ErrNoEngine represents an interpreter trying to be run without a backing engine.
var ErrNoEngine = errors.New("intrptr: interpreter has no associated engine")

// SignalHandler is a type that transforms incoming interrupt
// signals the UI has received.
type SignalHandler func(os.Signal) os.Signal

// Option represents setting an option for the interpreter UI.
type Option func(*UI)

// WithPrefix specifies the prefix
func WithPrefix(prefix string) Option {
	return func(ui *UI) {
		ui.prefix = []byte(prefix)
	}
}

// WithIO specifies the Reader and Writer to use for IO.
func WithIO(in io.Reader, out io.Writer) Option {
	return func(ui *UI) {
		ui.i = in
		ui.o = out
	}
}

// WithSignalHandlers specifies user provided signal handlers to register.
func WithSignalHandlers(handlers map[os.Signal]SignalHandler) Option {
	return func(ui *UI) {
		ui.sigHandlers = handlers
	}
}

// UI represents the user interface for the interpreter.
// UI will listens for Interrupt signals and
// handles them as graceful as possible. If
// signal handlers are provided then the handling of
// the Interrupt signal can be overwritten.
//
type UI struct {
	// Engine shit
	eng   Engine
	reqCh chan execReq

	// I/O shit
	i           io.Reader
	o           io.Writer
	prefix      []byte
	line        []byte
	sigs        chan os.Signal
	sigHandlers map[os.Signal]SignalHandler

	// Execution shit
	ctx context.Context
}

// NewUI returns a new user interface for an interpreter.
func NewUI(eng Engine, opts ...Option) *UI {
	ui := &UI{
		eng:   eng,
		reqCh: make(chan execReq),
		sigs:  make(chan os.Signal, 1),
	}
	signal.Notify(ui.sigs, os.Interrupt)

	for _, opt := range opts {
		opt(ui)
	}

	return ui
}

// SetPrefix sets the interpreters line prefix
func (ui *UI) SetPrefix(prefix string) {
	ui.prefix = []byte(prefix)
}

// SetIO sets the interpreters I/O.
func (ui *UI) SetIO(in io.Reader, out io.Writer) {
	ui.i = in
	ui.o = out
}

// Run creates a UI and associates the provided Engine to it.
// It then starts the UI. I/O must be provided for this call
// to not panic.
func Run(ctx context.Context, eng Engine, opts ...Option) error {
	if eng == nil {
		return ErrNoEngine
	}

	ui := NewUI(eng)
	return ui.Run(ctx, opts...)
}

// minRead
const minRead = 512

// Run starts the user interface with the provided sources
// for input and output of the interpreter and engine.
// The prefix will be printed before every line.
func (ui *UI) Run(ctx context.Context, opts ...Option) (err error) {
	// Make sure engine is set
	if ui.eng == nil {
		return ErrNoEngine
	}
	// Check if context is nil
	if ctx == nil {
		ctx = context.Background()
	}

	// Set options
	for _, opt := range opts {
		opt(ui)
	}

	// Set signal handling
	var cancel func()
	ui.ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	defer close(ui.sigs)
	go func() {
		for sig := range ui.sigs {
			handler, exists := ui.sigHandlers[sig]
			if exists {
				sig = handler(sig)
			}
			if sig == os.Kill || sig == os.Interrupt {
				cancel()
			}
			select {
			case <-ui.ctx.Done():
				return
			default:
			}
		}
	}()

	// Start engine
	ui.startEngine(ui.ctx, ui.eng)

	// Now, begin reading lines from input.
	defer func() {
		oldPre := ui.prefix
		ui.SetPrefix("\n")
		ui.Write(nil)
		ui.prefix = oldPre
	}()
	defer close(ui.reqCh)

	var n int
	for {
		// Write prefix
		_, err = ui.Write(nil)
		if err != nil {
			return
		}

		// Read line
		b := make([]byte, minRead)
		n, err = ui.Read(b)
		if err == context.Canceled {
			return nil
		}
		if err != nil && err != io.EOF || n == 0 {
			return
		}

		// Execute line
		status := ui.exec(ui.ctx, string(b))
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
func (ui *UI) startEngine(ctx context.Context, eng Engine) {
	if ui.reqCh == nil {
		ui.reqCh = make(chan execReq)
	}

	engines.Lock()
	reqCh, exists := engines.engs[eng]
	if !exists {
		reqCh = make(chan chan execReq)
		engines.engs[eng] = reqCh
		go runEngine(ctx, eng, reqCh)
	}
	engines.Unlock()

	reqCh <- ui.reqCh
}

type readResp struct {
	n   int
	err error
}

// readAsync wraps a Read call and sends the result to the give channel
func (ui *UI) readAsync(b []byte, readCh chan readResp) {
	var resp readResp
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
func (ui *UI) Read(b []byte) (n int, err error) {
	readCh := make(chan readResp, 1)

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

// Write writes the provided bytes to the UIs underlying
// output along with the prefix characters.
func (ui *UI) Write(b []byte) (n int, err error) {
	n, err = ui.o.Write(ui.prefix) // TODO: Handle prefix error properly
	if err == nil && b != nil {
		n, err = ui.o.Write(b)
	}
	return
}
