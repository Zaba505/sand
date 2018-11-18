package sand

import (
	"context"
	"io"
)

// Engine represents the command processor for the interpreter.
type Engine interface {
	// Exec should take the given line and execute the corresponding functionality.
	Exec(ctx context.Context, line string, ui io.ReadWriter) (status int)
}

type execReq struct {
	ctx    context.Context
	line   string
	ui     io.ReadWriter
	respCh chan int
}

// exec sends the given line to the backing engine and awaits the results.
// this is a blocking call.
func (ui *UI) exec(ctx context.Context, line string) int {
	req := execReq{
		ctx:    ctx,
		line:   line,
		ui:     ui,
		respCh: make(chan int),
	}
	defer close(req.respCh)

	ui.reqCh <- req

	return <-req.respCh
}

// runEngine provides a container for an engine to run inside.
func runEngine(ctx context.Context, eng Engine, reqChs chan chan execReq) {
	defer close(reqChs)

	for {
		select {
		case <-ctx.Done():
			return
		case reqCh := <-reqChs:
			go func(rc chan execReq) {
				for req := range rc {
					resp := eng.Exec(req.ctx, req.line, req.ui)
					req.respCh <- resp
				}
			}(reqCh)
		}
	}
}
