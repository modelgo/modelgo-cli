// Command modelgo-cli is the modelgo CLI entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/modelgo/modelgo-cli/internal/hello"
	"github.com/modelgo/modelgo-cli/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Println(version.Version)
	case "--help", "-h":
		printUsage(os.Stdout)
	case "hello":
		runHello(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func runHello(args []string) {
	fs := flag.NewFlagSet("hello", flag.ExitOnError)
	name := fs.String("name", "world", "name to greet")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	fmt.Println(hello.Greet(*name))
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `modelgo-cli — the official modelgo CLI

USAGE:
    modelgo-cli <command> [flags]

COMMANDS:
    hello [--name NAME]   Print a greeting
    --version, -v         Print the version
    --help, -h            Show this help`)
}
