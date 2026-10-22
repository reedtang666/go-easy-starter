package main

import (
	_ "github.com/reedtang666/go-easy-starter/internal/logic"

	"fmt"
	"os"

	"github.com/reedtang666/go-easy-starter/internal/cmd"
)

func main() {
	if e := cmd.Run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
