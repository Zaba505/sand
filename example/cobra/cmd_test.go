package main

import (
	"bytes"
	"context"
	"github.com/Zaba505/sand"
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

// ExecHandler provides the ability to write dynamic Exec calls which have
// access to the testing framework.
type ExecHandler func(t *testing.T) func(context.Context, string, io.ReadWriter) int

// CmdTester is an Engine that can be used for testing commands
type CmdTester struct {
	T *testing.T
	H ExecHandler
}

func (eng *CmdTester) Exec(ctx context.Context, line string, ui io.ReadWriter) int {
	return eng.H(eng.T)(ctx, line, ui)
}

// echoHandler wraps rootCmd in the testing framework.
func echoHandler(t *testing.T) func(context.Context, string, io.ReadWriter) int {
	return func(ctx context.Context, line string, ui io.ReadWriter) int {
		rootCmd.SetArgs(strings.Split(line, " "))
		rootCmd.SetOutput(ui)

		err := rootCmd.Execute()
		if err != nil {
			t.Error(err)
			return 1
		}

		return 0
	}
}

func TestRootCmd(t *testing.T) {
	testCases := []struct {
		Name  string
		In    string
		ExOut string
	}{
		{
			Name:  "TestHelloWorld",
			In:    "hello, world!",
			ExOut: "[hello, world!]",
		},
		{
			Name:  "TestGoodbyeWorld",
			In:    "goodbye, world!",
			ExOut: "[goodbye, world!]",
		},
	}

	for _, testCase := range testCases {
		inData := testCase.In
		outData := testCase.ExOut
		t.Run(testCase.Name, func(subT *testing.T) {
			var in, out bytes.Buffer

			eng := &CmdTester{
				T: subT,
				H: echoHandler,
			}

			ui := sand.NewUI(eng)

			ui.SetPrefix(">")
			ui.SetIO(&in, &out)

			rootCmd.Run = echo(ui)

			_, err := in.Write([]byte(inData))
			if err != nil {
				subT.Error(err)
			}

			if err = ui.Run(); err != io.EOF {
				subT.Error(err)
			}

			b, err := ioutil.ReadAll(&out)
			if err != nil {
				subT.Error(err)
			}

			b = bytes.Trim(b, ">")
			if string(append(b[:len(outData)-1], "]"...)) != outData {
				subT.Fail()
			}
		})
	}
}
