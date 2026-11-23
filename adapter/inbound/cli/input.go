package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
)

// inputFlags let lookup and save take their word from the desktop instead of
// an argument, and report results through a desktop notification. The
// notification path exists for global shortcuts, where no terminal is visible.
type inputFlags struct {
	selection bool
	clipboard bool
	notify    bool
}

func (f *inputFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.selection, "selection", false, "use the currently highlighted text (primary selection; Linux only)")
	cmd.Flags().BoolVar(&f.clipboard, "clipboard", false, "use the clipboard text instead (works on every platform)")
	cmd.Flags().BoolVar(&f.notify, "notify", false, "also show the result as a desktop notification")
}

func (f *inputFlags) validate(args []string) error {
	switch {
	case f.selection && f.clipboard:
		return errors.New("--selection and --clipboard cannot be combined")
	case (f.selection || f.clipboard) && len(args) > 0:
		return errors.New("give either a word or --selection/--clipboard, not both")
	case !f.selection && !f.clipboard && len(args) == 0:
		return errors.New("a word is required (or use --selection / --clipboard)")
	}
	return nil
}

func (f *inputFlags) term(ctx context.Context, svc Services, args []string) (string, error) {
	switch {
	case f.selection:
		return svc.Selection.CaptureTerm(ctx, inbound.SelectionPrimary)
	case f.clipboard:
		return svc.Selection.CaptureTerm(ctx, inbound.SelectionClipboard)
	}
	return termArg(args), nil
}

// send shows a notification without ever failing the command: the result has
// already been produced and printed.
func (f *inputFlags) send(ctx context.Context, env Env, title, body string) {
	if env.Notify == nil {
		fmt.Fprintln(env.Stderr, "vocab: notifications are not available in this build")
		return
	}
	if err := env.Notify(ctx, title, body); err != nil {
		fmt.Fprintf(env.Stderr, "vocab: could not show notification: %v\n", err)
	}
}

// notifyOnError runs fn and, in notify mode, also reports failures as a
// notification, because a shortcut-launched command has nowhere else to show
// them.
func (f *inputFlags) notifyOnError(ctx context.Context, env Env, fn func() error) error {
	err := fn()
	if err != nil && f.notify {
		f.send(ctx, env, "Vocabinator", err.Error())
	}
	return err
}

const maxNotificationRunes = 400

// summarizeLookup condenses a result into a notification title and body.
func summarizeLookup(r *inbound.LookupResult) (title, body string) {
	title = r.Term
	if r.Phonetic != "" {
		title += "  " + r.Phonetic
	}

	var lines []string
	if r.Translation != "" {
		lines = append(lines, "≈ "+r.Translation)
	}

	shown := 0
	for _, s := range r.Senses {
		if len(s.Definitions) == 0 || shown == 2 {
			continue
		}
		shown++
		d := s.Definitions[0]
		line := d.Text
		if s.PartOfSpeech != "" {
			line = s.PartOfSpeech + ": " + line
		}
		lines = append(lines, line)
		if d.Translation != "" {
			lines = append(lines, "→ "+d.Translation)
		}
	}
	if len(r.Warnings) > 0 && r.Translation == "" {
		lines = append(lines, "(translation unavailable)")
	}

	return title, truncateRunes(strings.Join(lines, "\n"), maxNotificationRunes)
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}
