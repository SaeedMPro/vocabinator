// Package cli is the Cobra command line adapter. It only parses input, calls
// inbound ports and renders results; it holds no business logic.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// Services are the inbound ports the CLI drives, plus user defaults.
type Services struct {
	Vocabulary inbound.VocabularyService
	SaveWord   inbound.SaveWordService
	Lookup     inbound.LookupService
	Selection  inbound.SelectionService

	DefaultSource vocab.LanguageCode
	DefaultTarget vocab.LanguageCode
}

// Env supplies I/O and lazily opens services, so `--help` and flag errors
// never touch the database.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Open   func(ctx context.Context) (Services, io.Closer, error)

	// Version is shown by --version.
	Version string

	// Notify shows a desktop notification. Optional: without it --notify
	// reports that notifications are unavailable.
	Notify func(ctx context.Context, title, body string) error

	// Serve runs the local web interface until ctx is cancelled. When nil the
	// serve command is not registered.
	Serve func(ctx context.Context, addr string, out io.Writer) error
}

func NewRootCmd(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "vocab",
		Short:         "Turn unknown words into knowledge.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       env.Version,
	}
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	root.AddCommand(
		newLookupCmd(env),
		newSaveCmd(env),
		newListCmd(env),
		newShowCmd(env),
		newRemoveCmd(env),
	)
	if env.Serve != nil {
		root.AddCommand(newServeCmd(env))
	}
	return root
}

// run opens services for the duration of one command.
func run(env Env, fn func(ctx context.Context, svc Services) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		svc, closer, err := env.Open(cmd.Context())
		if err != nil {
			return err
		}
		defer closer.Close()
		return fn(cmd.Context(), svc)
	}
}

// langFlags are the shared --source/--target flags.
type langFlags struct{ source, target string }

func (f *langFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.source, "source", "", "source language code (default from "+"VOCAB_SOURCE_LANGUAGE, else en)")
	cmd.Flags().StringVar(&f.target, "target", "", "target language code (default from "+"VOCAB_TARGET_LANGUAGE, else fa)")
}

func (f *langFlags) resolve(svc Services) (source, target vocab.LanguageCode, err error) {
	source, target = svc.DefaultSource, svc.DefaultTarget
	if f.source != "" {
		if source, err = vocab.NewLanguageCode(f.source); err != nil {
			return "", "", fmt.Errorf("--source %q: %w", f.source, err)
		}
	}
	if f.target != "" {
		if target, err = vocab.NewLanguageCode(f.target); err != nil {
			return "", "", fmt.Errorf("--target %q: %w", f.target, err)
		}
	}
	return source, target, nil
}

// optionalLang parses a filter flag that has no default.
func optionalLang(flag, value string) (*vocab.LanguageCode, error) {
	if value == "" {
		return nil, nil
	}
	code, err := vocab.NewLanguageCode(value)
	if err != nil {
		return nil, fmt.Errorf("--%s %q: %w", flag, value, err)
	}
	return &code, nil
}

func termArg(args []string) string { return strings.Join(args, " ") }

type format string

const (
	formatText format = "text"
	formatJSON format = "json"
)

func parseFormat(v string) (format, error) {
	switch format(v) {
	case formatText, formatJSON:
		return format(v), nil
	}
	return "", fmt.Errorf("--format %q: must be text or json", v)
}
