// Command lab builds the agent laboratory with the real models and serves it on
// https://localhost:8443 (README, "Laboratorio").
package main

import (
	"fmt"
	"os"

	"webtyp.com/agenteval/lab"
)

func main() {
	if err := lab.Serve(".", lab.Port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
