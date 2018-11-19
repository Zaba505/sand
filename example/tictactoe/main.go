package main

import (
	"context"
	"fmt"
	"github.com/Zaba505/sand"
	"io"
	"log"
	"os"
	"unicode/utf8"
)

// Player represents X or O
type Player uint8

// players
const (
	Nan Player = iota
	X
	O
)

func (p Player) String() string {
	switch p {
	case Nan:
		return " "
	case X:
		return "X"
	case O:
		return "O"
	}
	return "NoPlayer"
}

// T3Engine implements a Tic-Tac-Toe game engine.
// It is triggered by one command: tictactoe.
type T3Engine struct {
	board [][]Player
}

// Exec starts a Tic-Tac-Toe game
func (eng *T3Engine) Exec(ctx context.Context, line string, ui io.ReadWriter) int {
	// Command 'tictactoe' is the only valid triggerer
	if line[:9] != "tictactoe" {
		return 1
	}

	// Create board
	eng.board = make([][]Player, 3)
	for i := range eng.board {
		eng.board[i] = make([]Player, 3)
	}

	// Play
	curPlayer := X
	for !eng.isOver() {
		// Print game board
		eng.printBoard(ui)
		_, err := ui.Write(nil)
		if err != nil {
			log.Println(err)
			return 1
		}

		// Next, get user position input
		b := make([]byte, 2)
		_, err = ui.Read(b)
		if err == context.Canceled {
			return 1
		}
		if err != nil {
			log.Println(err)
			return 1
		}
		r, _ := utf8.DecodeRune(b)

		// Update board
		switch r {
		case '1':
			eng.board[2][0] = curPlayer
		case '2':
			eng.board[2][1] = curPlayer
		case '3':
			eng.board[2][2] = curPlayer
		case '4':
			eng.board[1][0] = curPlayer
		case '5':
			eng.board[1][1] = curPlayer
		case '6':
			eng.board[1][2] = curPlayer
		case '7':
			eng.board[0][0] = curPlayer
		case '8':
			eng.board[0][1] = curPlayer
		case '9':
			eng.board[0][2] = curPlayer
		default:
			select {
			case <-ctx.Done():
				return 0
			default:
			}
			fmt.Fprintln(ui, "Invalid position:", r)
			// TODO: Add retry logic
		}

		// Switch players
		if curPlayer == X {
			curPlayer = O
		} else {
			curPlayer = X
		}
	}
	eng.printBoard(ui)

	// Print ending message
	winner, ok := hasWinner(eng.board)
	if ok {
		fmt.Fprintf(ui, "Player %s won!\n", winner)
	} else {
		fmt.Fprintln(ui, "This game is a tie!")
	}

	return 0
}

func (eng *T3Engine) printBoard(w io.Writer) {
	ui, _ := w.(*sand.UI)
	ui.SetPrefix("")
	defer func() {
		ui.SetPrefix(">")
	}()

	fmt.Fprintf(w, ` %s | %s | %s
-----------
 %s | %s | %s
-----------
 %s | %s | %s
`, eng.board[0][0], eng.board[0][1], eng.board[0][2],
		eng.board[1][0], eng.board[1][1], eng.board[1][2],
		eng.board[2][0], eng.board[2][1], eng.board[2][2])
}

func (eng *T3Engine) isOver() bool {
	_, hasWinner := hasWinner(eng.board)
	full := eng.board[0][0] != Nan && eng.board[0][1] != Nan && eng.board[0][2] != Nan &&
		eng.board[1][0] != Nan && eng.board[1][1] != Nan && eng.board[1][2] != Nan &&
		eng.board[2][0] != Nan && eng.board[2][1] != Nan && eng.board[2][2] != Nan
	return hasWinner || full
}

func hasWinner(board [][]Player) (player Player, ok bool) {
	for _, player = range []Player{X, O} {
		switch {
		case player == board[0][0] && player == board[0][1] && player == board[0][2]:
		case player == board[1][0] && player == board[1][1] && player == board[1][2]:
		case player == board[2][0] && player == board[2][1] && player == board[2][2]:
		case player == board[0][0] && player == board[1][0] && player == board[2][0]:
		case player == board[0][1] && player == board[1][1] && player == board[2][1]:
		case player == board[0][2] && player == board[1][2] && player == board[2][2]:
		case player == board[0][0] && player == board[1][1] && player == board[2][2]:
		case player == board[0][2] && player == board[1][1] && player == board[2][0]:
		default:
			continue
		}
		ok = true
		return
	}
	return Nan, false
}

func main() {
	ui := sand.NewUI(new(T3Engine))

	ui.SetPrefix(">")
	ui.SetIO(os.Stdin, os.Stdout)

	if err := ui.Run(nil); err != nil {
		log.Fatal(err)
	}
}
