package cli

import (
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
)

func TestSummarizeLookup(t *testing.T) {
	r := &inbound.LookupResult{
		Term: "robust", Phonetic: "/x/", Translation: "قوی",
		Senses: []inbound.SenseResult{
			{PartOfSpeech: "adjective", Definitions: []inbound.DefinitionResult{{Text: "Strong.", Translation: "نیرومند"}, {Text: "second"}}},
			{PartOfSpeech: "noun", Definitions: []inbound.DefinitionResult{{Text: "A thing."}}},
			{PartOfSpeech: "verb", Definitions: []inbound.DefinitionResult{{Text: "third sense is skipped"}}},
		},
	}
	title, body := summarizeLookup(r)

	if title != "robust  /x/" {
		t.Errorf("title %q", title)
	}
	for _, want := range []string{"≈ قوی", "adjective: Strong.", "→ نیرومند", "noun: A thing."} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "second") || strings.Contains(body, "third") {
		t.Errorf("body must only include the first definition of the first two senses:\n%s", body)
	}
}

func TestSummarizeLookupWarningAndTruncation(t *testing.T) {
	_, body := summarizeLookup(&inbound.LookupResult{
		Term:     "x",
		Senses:   []inbound.SenseResult{{Definitions: []inbound.DefinitionResult{{Text: "d"}}}},
		Warnings: []string{"translation unavailable: boom"},
	})
	if !strings.Contains(body, "(translation unavailable)") || strings.Contains(body, "boom") {
		t.Errorf("unexpected body %q", body)
	}

	long := strings.Repeat("é", 1000)
	_, body = summarizeLookup(&inbound.LookupResult{Term: "x", Senses: []inbound.SenseResult{{Definitions: []inbound.DefinitionResult{{Text: long}}}}})
	if n := len([]rune(body)); n > maxNotificationRunes || !strings.HasSuffix(body, "…") {
		t.Errorf("body not truncated safely: %d runes", n)
	}
}
