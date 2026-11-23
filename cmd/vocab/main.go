// Command vocab is the Vocabinator command line interface.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/SaeedMPro/vocabinator/adapter/inbound/cli"
	"github.com/SaeedMPro/vocabinator/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := cli.NewRootCmd(cli.Env{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Open: func(ctx context.Context) (cli.Services, io.Closer, error) {
			cfg, err := bootstrap.LoadConfig(os.Getenv)
			if err != nil {
				return cli.Services{}, nil, err
			}
			app, err := bootstrap.Build(ctx, cfg)
			if err != nil {
				return cli.Services{}, nil, err
			}
			return cli.Services{
				Vocabulary:    app.Vocabulary,
				SaveWord:      app.SaveWord,
				Lookup:        app.Lookup,
				DefaultSource: cfg.SourceLanguage,
				DefaultTarget: cfg.TargetLanguage,
			}, app, nil
		},
	})

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "vocab:", err)
		os.Exit(1)
	}
}
