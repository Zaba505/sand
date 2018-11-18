package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "echo",
	Short: "Echo back args",
}

// echo echos the given args back out
func echo(ui io.ReadWriter) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(ui, args)
	}
}

func main() {
	rootCmd.Run = echo(os.Stdout) // Doesn't need to do more reads

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
