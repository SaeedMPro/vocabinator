// Command vocab is the Vocabinator command line interface.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/SaeedMPro/vocabinator/adapter/inbound/cli"
	"github.com/SaeedMPro/vocabinator/adapter/inbound/web"
	"github.com/SaeedMPro/vocabinator/adapter/platform/notify"
	"github.com/SaeedMPro/vocabinator/bootstrap"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	root := cli.NewRootCmd(cli.Env{
		Serve:   serve,
		Version: version,
		Notify:  notify.New().Notify,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
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
				Selection:     app.Selection,
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

// serve wires the web adapter to the application and runs it.
func serve(ctx context.Context, addr string, out io.Writer) error {
	// Fail on a non-loopback address before touching the database.
	ln, err := web.ListenLoopback(addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	cfg, err := bootstrap.LoadConfig(os.Getenv)
	if err != nil {
		return err
	}
	app, err := bootstrap.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.Close()

	token, err := web.NewToken()
	if err != nil {
		return err
	}

	srv, err := web.NewServer(web.Deps{
		Vocabulary:    app.Vocabulary,
		SaveWord:      app.SaveWord,
		Lookup:        app.Lookup,
		DefaultSource: cfg.SourceLanguage,
		DefaultTarget: cfg.TargetLanguage,
		Settings: web.Settings{
			DatabasePath:        cfg.DatabasePath,
			TranslationProvider: cfg.TranslationLabel(),
		},
		Token: token,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Vocabinator is running. Open this URL in your browser (it contains your access token):\n\n  http://%s/?token=%s\n\nPress Ctrl+C to stop.\n", ln.Addr(), token)
	return web.Serve(ctx, ln, srv.Handler())
}
