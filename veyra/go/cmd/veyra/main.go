// Command veyra is the adaptive execution kernel CLI.
package main

import (
	"os"

	"github.com/nisaral/veyra/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		os.Stderr.WriteString("veyra: " + err.Error() + "\n")
		os.Exit(1)
	}
}