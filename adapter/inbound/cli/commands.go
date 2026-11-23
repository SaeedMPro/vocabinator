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
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "lookup <word>",
		Short: "Look up a word without saving it",
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
			result, err := svc.Lookup.LookupWord(ctx, inbound.LookupWordInput{
				Term: termArg(args), SourceLanguage: source, TargetLanguage: target,
			})
			if err != nil {
				return friendly(err)
			}
			return renderLookup(env.Stdout, f, result)
		})(cmd, args)
	}
	return cmd
}

func newSaveCmd(env Env) *cobra.Command {
	var langs langFlags
	var formatFlag, contextFlag, urlFlag string

	cmd := &cobra.Command{
		Use:   "save <word>",
		Short: "Save a word, optionally with the context it was seen in",
		Args:  cobra.MinimumNArgs(1),
	}
	langs.register(cmd)
	cmd.Flags().StringVar(&formatFlag, "format", "text", "output format: text or json")
	cmd.Flags().StringVar(&contextFlag, "context", "", "sentence or passage where the word was seen")
	cmd.Flags().StringVar(&urlFlag, "url", "", "source URL of the context")

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
			result, err := svc.SaveWord.SaveWord(ctx, inbound.SaveWordInput{
				Term: termArg(args), SourceLanguage: source, TargetLanguage: target,
				Context: contextFlag, SourceURL: urlFlag,
			})
			if err != nil {
				return friendly(err)
			}

			if f == formatText {
				switch {
				case result.Created:
					fmt.Fprintf(env.Stdout, "Saved %q.\n", result.Item.Term())
				case result.Occurrence != nil:
					fmt.Fprintf(env.Stdout, "%q was already saved; added a new context.\n", result.Item.Term())
				default:
					fmt.Fprintf(env.Stdout, "%q was already saved.\n", result.Item.Term())
				}
				if result.Created && result.Occurrence != nil {
					fmt.Fprintln(env.Stdout, "Context recorded.")
				}
				return nil
			}
			return renderItem(env.Stdout, f, result.Item)
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
	}
	return err
}
