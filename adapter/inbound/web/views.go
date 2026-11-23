package web

import (
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type languageOption struct {
	Code string
	Name string
}

func languageOptions() []languageOption {
	var out []languageOption
	for _, l := range vocab.SupportedLanguages() {
		out = append(out, languageOption{Code: l.String(), Name: l.DisplayName()})
	}
	return out
}

type wordSummary struct {
	ID        string
	Term      string
	Source    string
	Target    string
	Contexts  int
	CreatedAt time.Time
}

func toSummaries(items []*vocab.VocabularyItem) []wordSummary {
	out := make([]wordSummary, 0, len(items))
	for _, item := range items {
		out = append(out, wordSummary{
			ID: string(item.ID()), Term: item.Term(),
			Source: item.SourceLanguage().String(), Target: item.TargetLanguage().String(),
			Contexts: len(item.Occurrences()), CreatedAt: item.CreatedAt(),
		})
	}
	return out
}

type pairCount struct {
	Source string
	Target string
	Count  int
}

type dashboardView struct {
	Total         int
	TotalContexts int
	Pairs         []pairCount
	Recent        []wordSummary
	Languages     []languageOption
	Source        string
	Target        string
}

func (s *Server) newDashboardView(items []*vocab.VocabularyItem) dashboardView {
	v := dashboardView{
		Total:     len(items),
		Languages: languageOptions(),
		Source:    s.deps.DefaultSource.String(),
		Target:    s.deps.DefaultTarget.String(),
	}

	pairs := map[[2]string]int{}
	for _, item := range items {
		v.TotalContexts += len(item.Occurrences())
		pairs[[2]string{item.SourceLanguage().String(), item.TargetLanguage().String()}]++
	}
	for k, n := range pairs {
		v.Pairs = append(v.Pairs, pairCount{Source: k[0], Target: k[1], Count: n})
	}
	sort.Slice(v.Pairs, func(i, j int) bool {
		if v.Pairs[i].Count != v.Pairs[j].Count {
			return v.Pairs[i].Count > v.Pairs[j].Count
		}
		return v.Pairs[i].Source+v.Pairs[i].Target < v.Pairs[j].Source+v.Pairs[j].Target
	})

	recent := items
	if len(recent) > recentWordCount {
		recent = recent[:recentWordCount]
	}
	v.Recent = toSummaries(recent)
	return v
}

type wordsView struct {
	Query     string
	Source    string
	Target    string
	Languages []languageOption
	Words     []wordSummary
	Page      int
	PrevURL   string
	NextURL   string
}

type occurrenceView struct {
	ID         string
	Context    string
	SourceURL  string
	LinkURL    string // empty unless SourceURL is a safe http(s) URL
	CapturedAt time.Time
}

type wordView struct {
	ID          string
	Term        string
	Source      string
	Target      string
	SourceName  string
	TargetName  string
	CreatedAt   time.Time
	Occurrences []occurrenceView
}

func toWordView(item *vocab.VocabularyItem) wordView {
	v := wordView{
		ID: string(item.ID()), Term: item.Term(),
		Source: item.SourceLanguage().String(), Target: item.TargetLanguage().String(),
		SourceName: item.SourceLanguage().DisplayName(), TargetName: item.TargetLanguage().DisplayName(),
		CreatedAt: item.CreatedAt(),
	}
	for _, o := range item.Occurrences() {
		v.Occurrences = append(v.Occurrences, occurrenceView{
			ID: string(o.ID()), Context: o.Context(), SourceURL: o.SourceURL(),
			LinkURL: safeLink(o.SourceURL()), CapturedAt: o.CapturedAt(),
		})
	}
	return v
}

// safeLink returns raw only when it is an absolute http or https URL, so
// stored text can never become a javascript: or data: link.
func safeLink(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}

type settingsView struct {
	DatabasePath          string
	TranslationConfigured bool
	Source                vocab.LanguageCode
	Target                vocab.LanguageCode
}

type errorView struct {
	Status  int
	Message string
}

// JSON contract for /api/lookup.

type lookupJSON struct {
	Term        string      `json:"term"`
	Phonetic    string      `json:"phonetic,omitempty"`
	Translation string      `json:"translation,omitempty"`
	Senses      []senseJSON `json:"senses"`
	Warnings    []string    `json:"warnings,omitempty"`
}

type senseJSON struct {
	PartOfSpeech string           `json:"partOfSpeech,omitempty"`
	Definitions  []definitionJSON `json:"definitions"`
	Examples     []string         `json:"examples,omitempty"`
	Synonyms     []string         `json:"synonyms,omitempty"`
	Antonyms     []string         `json:"antonyms,omitempty"`
}

type definitionJSON struct {
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
}

func toLookupJSON(r *inbound.LookupResult) lookupJSON {
	out := lookupJSON{Term: r.Term, Phonetic: r.Phonetic, Translation: r.Translation, Warnings: r.Warnings, Senses: []senseJSON{}}
	for _, s := range r.Senses {
		sj := senseJSON{PartOfSpeech: s.PartOfSpeech, Examples: s.Examples, Synonyms: s.Synonyms, Antonyms: s.Antonyms, Definitions: []definitionJSON{}}
		for _, d := range s.Definitions {
			sj.Definitions = append(sj.Definitions, definitionJSON{Text: d.Text, Translation: d.Translation})
		}
		out.Senses = append(out.Senses, sj)
	}
	return out
}
