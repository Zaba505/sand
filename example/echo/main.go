package main

import (
	"context"
	"github.com/Zaba505/sand"
	"io"
	"log"
	"os"
)

// EchoEngine simply echos the given line
type EchoEngine struct{}

// Exec simply writes the given line back to the ui
func (eng *EchoEngine) Exec(ctx context.Context, line string, ui io.ReadWriter) (status int) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	_, err := ui.Write([]byte(line))
	if err != nil {
		log.Printf("error encountered: %s\n", err)
		return 1
	}
	return
}

func main() {
	ui := sand.NewUI(new(EchoEngine))

	log.SetOutput(os.Stdout)
	err := ui.Run(
		nil,
		sand.WithPrefix(">"),
		sand.WithIO(os.Stdin, os.Stdout),
	)
	if err != nil {
		log.Fatal(err)
	}
}
