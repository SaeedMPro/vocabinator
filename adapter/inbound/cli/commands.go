package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/usecase"
)

func newLookupCmd(env Env) *cobra.Command {
	var langs langFlags
	var in inputFlags
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "lookup [word]",
		Short: "Look up a word without saving it",
		Long: "Look up a word without saving it. Give the word as an argument, or use " +
			"--selection to look up the text currently highlighted in any application.",
		Args: cobra.ArbitraryArgs,
	}
	langs.register(cmd)
	in.register(cmd)
	cmd.Flags().StringVar(&formatFlag, "format", "text", "output format: text or json")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		f, err := parseFormat(formatFlag)
		if err != nil {
			return err
		}
		if err := in.validate(args); err != nil {
			return err
		}
		return run(env, func(ctx context.Context, svc Services) error {
			return in.notifyOnError(ctx, env, func() error {
				source, target, err := langs.resolve(svc)
				if err != nil {
					return err
				}
				term, err := in.term(ctx, svc, args)
				if err != nil {
					return friendly(err)
				}
				result, err := svc.Lookup.LookupWord(ctx, inbound.LookupWordInput{
					Term: term, SourceLanguage: source, TargetLanguage: target,
				})
				if err != nil {
					return friendly(err)
				}
				if err := renderLookup(env.Stdout, f, result); err != nil {
					return err
				}
				if in.notify {
					title, body := summarizeLookup(result)
					in.send(ctx, env, title, body)
				}
				return nil
			})
		})(cmd, args)
	}
	return cmd
}

func newSaveCmd(env Env) *cobra.Command {
	var langs langFlags
	var in inputFlags
	var formatFlag, contextFlag, urlFlag string

	cmd := &cobra.Command{
		Use:   "save [word]",
		Short: "Save a word, optionally with the context it was seen in",
		Long: "Save a word, optionally with the context it was seen in. Give the word as an " +
			"argument, or use --selection to save the text currently highlighted in any application.",
		Args: cobra.ArbitraryArgs,
	}
	langs.register(cmd)
	in.register(cmd)
	cmd.Flags().StringVar(&formatFlag, "format", "text", "output format: text or json")
	cmd.Flags().StringVar(&contextFlag, "context", "", "sentence or passage where the word was seen")
	cmd.Flags().StringVar(&urlFlag, "url", "", "source URL of the context")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		f, err := parseFormat(formatFlag)
		if err != nil {
			return err
		}
		if err := in.validate(args); err != nil {
			return err
		}
		return run(env, func(ctx context.Context, svc Services) error {
			return in.notifyOnError(ctx, env, func() error {
				source, target, err := langs.resolve(svc)
				if err != nil {
					return err
				}
				term, err := in.term(ctx, svc, args)
				if err != nil {
					return friendly(err)
				}
				result, err := svc.SaveWord.SaveWord(ctx, inbound.SaveWordInput{
					Term: term, SourceLanguage: source, TargetLanguage: target,
					Context: contextFlag, SourceURL: urlFlag,
				})
				if err != nil {
					return friendly(err)
				}

				var message string
				switch {
				case result.Created && result.Occurrence != nil:
					message = fmt.Sprintf("Saved %q.\nContext recorded.", result.Item.Term())
				case result.Created:
					message = fmt.Sprintf("Saved %q.", result.Item.Term())
				case result.Occurrence != nil:
					message = fmt.Sprintf("%q was already saved; added a new context.", result.Item.Term())
				default:
					message = fmt.Sprintf("%q was already saved.", result.Item.Term())
				}

				if f == formatText {
					fmt.Fprintln(env.Stdout, message)
				} else if err := renderItem(env.Stdout, f, result.Item); err != nil {
					return err
				}
				if in.notify {
					in.send(ctx, env, "Vocabinator", message)
				}
				return nil
			})
		})(cmd, args)
	}
	return cmd
}

func newListCmd(env Env) *cobra.Command {
	var source, target, search, formatFlag string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved vocabulary",
		Args:  cobra.NoArgs,
	}
	cmd.Flags().StringVar(&source, "source", "", "only words with this source language")
	cmd.Flags().StringVar(&target, "target", "", "only words with this target language")
	cmd.Flags().StringVar(&search, "search", "", "only words containing this text")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of words (0 for all)")
	cmd.Flags().StringVar(&formatFlag, "format", "text", "output format: text or json")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		f, err := parseFormat(formatFlag)
		if err != nil {
			return err
		}
		src, err := optionalLang("source", source)
		if err != nil {
			return err
		}
		tgt, err := optionalLang("target", target)
		if err != nil {
			return err
		}
		return run(env, func(ctx context.Context, svc Services) error {
			items, err := svc.Vocabulary.ListVocabItems(ctx, inbound.VocabItemFilter{
				Term: search, SourceLanguage: src, TargetLanguage: tgt, Limit: limit,
			})
			if err != nil {
				return friendly(err)
			}
			return renderList(env.Stdout, f, items)
		})(cmd, args)
	}
	return cmd
}

func newShowCmd(env Env) *cobra.Command {
	var langs langFlags
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "show <word>",
		Short: "Show a saved word and the contexts it was seen in",
		Args:  cobra.MinimumNArgs(1),
	}
	langs.register(cmd)
	cmd.Flags().StringVar(&formatFlag, "format", "text", "output format: text or json")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		f, err := parseFormat(formatFlag)
		if err != nil {
			return err
		}
		return run(env, func(ctx context.Context, svc Services) error {
			source, target, err := langs.resolve(svc)
			if err != nil {
				return err
			}
			item, err := svc.Vocabulary.FindVocabItem(ctx, inbound.VocabItemKey{
				Term: termArg(args), SourceLanguage: source, TargetLanguage: target,
			})
			if err != nil {
				return friendly(err)
			}
			return renderItem(env.Stdout, f, item)
		})(cmd, args)
	}
	return cmd
}

func newRemoveCmd(env Env) *cobra.Command {
	var langs langFlags

	cmd := &cobra.Command{
		Use:   "remove <word>",
		Short: "Remove a saved word and its contexts",
		Args:  cobra.MinimumNArgs(1),
	}
	langs.register(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return run(env, func(ctx context.Context, svc Services) error {
			source, target, err := langs.resolve(svc)
			if err != nil {
				return err
			}
			item, err := svc.Vocabulary.FindVocabItem(ctx, inbound.VocabItemKey{
				Term: termArg(args), SourceLanguage: source, TargetLanguage: target,
			})
			if err != nil {
				return friendly(err)
			}
			if err := svc.Vocabulary.DeleteVocabItem(ctx, item.ID()); err != nil {
				return friendly(err)
			}
			fmt.Fprintf(env.Stdout, "Removed %q.\n", item.Term())
			return nil
		})(cmd, args)
	}
	return cmd
}

// friendly turns use-case errors into messages suitable for a terminal while
// keeping the original error in the chain.
func friendly(err error) error {
	switch {
	case errors.Is(err, usecase.ErrWordNotFound):
		return fmt.Errorf("no dictionary entry found for that word: %w", err)
	case errors.Is(err, usecase.ErrItemNotFound):
		return fmt.Errorf("that word is not in your vocabulary: %w", err)
	case errors.Is(err, usecase.ErrNoSelection):
		return fmt.Errorf("no text is selected: highlight a word first, or use --clipboard to read copied text: %w", err)
	}
	return err
}

// DefaultServeAddr is loopback-only by design.
const DefaultServeAddr = "127.0.0.1:8787"

func newServeCmd(env Env) *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local web interface",
		Long: "Start the local web interface. It only listens on loopback addresses and " +
			"prints a one-time URL containing the access token; open that URL in your browser.",
		Args: cobra.NoArgs,
	}
	cmd.Flags().StringVar(&addr, "addr", DefaultServeAddr, "loopback address to listen on (127.0.0.1, ::1 or localhost)")

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return env.Serve(cmd.Context(), addr, env.Stdout)
	}
	return cmd
}
