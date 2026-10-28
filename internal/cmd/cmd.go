package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/reedtang666/go-easy-starter/internal/bootstrap"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func Run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "module", "db-new", "db-status", "db-up", "db-baseline", "db-init":
			return runTool(args[0], args[1:])
		}
	}
	role := "all"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		role = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("starter", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: starter [all|http|queue|scheduler|setup] [--config PATH]")
		fmt.Fprintln(fs.Output(), "Development: module, db-new, db-status, db-up, db-baseline, db-init (COMMAND --help)")
		fs.PrintDefaults()
	}
	path := fs.String("config", "manifest/config/config.yaml", "configuration path")
	if e := fs.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if role == "setup" {
		return config.Setup(*path)
	}
	c, e := config.Load(*path)
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app, e := bootstrap.New(ctx, c, role)
	if e != nil {
		return e
	}
	defer app.Close()
	if e = app.Start(); e != nil {
		return e
	}
	fmt.Printf("starter role=%s started\n", role)
	<-ctx.Done()
	return nil
}
