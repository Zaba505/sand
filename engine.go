package sand

import (
	"context"
	"io"
)

// Engine represents the command processor for the interpreter.
// The underlying type of the Engine implementation must be a
// hashable type (e.g. int, string, struct) in order for the UI
// to be able to use it. Sadly, this means a type EngineFunc
// can not be used due to funcs not being hashable.
type Engine interface {
	// Exec should take the given line and execute the corresponding functionality.
	Exec(ctx context.Context, line string, ui io.ReadWriter) (status int)
}

// execReq represents the parameters passed to an Engine.Exec call
type execReq struct {
	ctx    context.Context
	line   string
	ui     io.ReadWriter
	respCh chan int
}

// exec sends the given line to the backing engine and awaits the results.
// this is a blocking call.
func (ui *UI) exec(ctx context.Context, line string, reqCh chan execReq) int {
	req := execReq{
		ctx:    ctx,
		line:   line,
		ui:     ui,
		respCh: make(chan int),
	}
	select {
	case <-ctx.Done():
		return 0
	case reqCh <- req:
	}
	return <-req.respCh
}

// runEngine provides a container for an engine to run inside.
func runEngine(ctx context.Context, eng Engine, reqChs chan chan execReq) {
	defer func() {
		close(reqChs)
		engines.Lock()
		delete(engines.engs, eng)
		engines.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case reqCh := <-reqChs:
			go func(rc chan execReq) {
				for req := range rc {
					resp := eng.Exec(req.ctx, req.line, req.ui)
					select {
					case <-ctx.Done():
						close(req.respCh)
						return
					case req.respCh <- resp:
					}
					close(req.respCh)
				}
			}(reqCh)
		}
	}
}
