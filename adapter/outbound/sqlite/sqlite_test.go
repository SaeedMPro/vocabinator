package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

var t0 = time.Date(2026, 11, 22, 12, 0, 0, 123456789, time.UTC)

func openTemp(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "vocab.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func newItem(t *testing.T, id, term string, src, tgt vocab.LanguageCode) *vocab.VocabularyItem {
	t.Helper()
	item, err := vocab.NewVocabularyItem(vocab.ID(id), term, src, tgt, t0)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func addOcc(t *testing.T, item *vocab.VocabularyItem, id, text string, at time.Time) vocab.Occurrence {
	t.Helper()
	o, err := vocab.NewOccurrence(vocab.ID(id), item.ID(), text, "https://example.com", at)
	if err != nil {
		t.Fatal(err)
	}
	if err := item.AddOccurrence(o); err != nil {
		t.Fatal(err)
	}
	return o
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSaveAndFindRoundTrip(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)
	ctx := context.Background()

	item := newItem(t, "i1", "  Robust ", "en", "fa")
	addOcc(t, item, "o1", "We need a robust architecture.", t0.Add(time.Minute))
	addOcc(t, item, "o2", "Go provides robust concurrency.", t0.Add(2*time.Minute))

	if err := repo.Save(ctx, item); err != nil {
		t.Fatal(err)
	}

	for name, find := range map[string]func() (*vocab.VocabularyItem, bool, error){
		"by id":  func() (*vocab.VocabularyItem, bool, error) { return repo.FindByID(ctx, "i1") },
		"by key": func() (*vocab.VocabularyItem, bool, error) { return repo.FindByKey(ctx, item.Key()) },
	} {
		got, ok, err := find()
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", name, ok, err)
		}
		if got.Term() != "Robust" || got.NormalizedTerm() != "robust" ||
			got.SourceLanguage() != "en" || got.TargetLanguage() != "fa" || !got.CreatedAt().Equal(t0) {
			t.Errorf("%s: unexpected item %+v", name, got)
		}
		occ := got.Occurrences()
		if len(occ) != 2 || occ[0].ID() != "o1" || occ[1].ID() != "o2" ||
			occ[0].Context() != "We need a robust architecture." || !occ[1].CapturedAt().Equal(t0.Add(2*time.Minute)) {
			t.Errorf("%s: unexpected occurrences %+v", name, occ)
		}
	}
}

func TestFindMissing(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)

	if _, ok, err := repo.FindByID(context.Background(), "nope"); ok || err != nil {
		t.Errorf("expected (false, nil), got ok=%v err=%v", ok, err)
	}
}

func TestSaveSynchronisesOccurrences(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)
	ctx := context.Background()

	item := newItem(t, "i1", "robust", "en", "fa")
	addOcc(t, item, "o1", "one", t0)
	addOcc(t, item, "o2", "two", t0.Add(time.Second))
	if err := repo.Save(ctx, item); err != nil {
		t.Fatal(err)
	}

	if err := item.RemoveOccurrence("o1"); err != nil {
		t.Fatal(err)
	}
	addOcc(t, item, "o3", "three", t0.Add(2*time.Second))
	if err := repo.Save(ctx, item); err != nil {
		t.Fatal(err)
	}

	got, _, _ := repo.FindByID(ctx, "i1")
	occ := got.Occurrences()
	if len(occ) != 2 || occ[0].ID() != "o2" || occ[1].ID() != "o3" {
		t.Errorf("unexpected occurrences: %+v", occ)
	}

	if err := item.RemoveOccurrence("o2"); err != nil {
		t.Fatal(err)
	}
	if err := item.RemoveOccurrence("o3"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "occurrences"); n != 0 {
		t.Errorf("expected 0 occurrences, got %d", n)
	}
}

func TestUniqueKeyConflict(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)
	ctx := context.Background()

	if err := repo.Save(ctx, newItem(t, "i1", "Robust", "en", "fa")); err != nil {
		t.Fatal(err)
	}

	err := repo.Save(ctx, newItem(t, "i2", " robust ", "en", "fa"))
	if !errors.Is(err, outbound.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// Different language pair is a different identity.
	if err := repo.Save(ctx, newItem(t, "i3", "robust", "en", "en")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n := count(t, db, "vocabulary_items"); n != 2 {
		t.Errorf("expected 2 items, got %d", n)
	}
}

func TestSaveIsTransactional(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)
	ctx := context.Background()

	owner := newItem(t, "i1", "robust", "en", "fa")
	addOcc(t, owner, "o1", "x", t0)
	if err := repo.Save(ctx, owner); err != nil {
		t.Fatal(err)
	}

	// A conflicting save must not leak the new item's occurrences.
	dup := newItem(t, "i2", "robust", "en", "fa")
	addOcc(t, dup, "o2", "y", t0)
	if err := repo.Save(ctx, dup); err == nil {
		t.Fatal("expected conflict")
	}
	if n := count(t, db, "occurrences"); n != 1 {
		t.Errorf("failed save leaked occurrences: have %d", n)
	}

	// Occurrence ID clash with another aggregate must not move or overwrite it.
	other := newItem(t, "i3", "fragile", "en", "fa")
	addOcc(t, other, "o1", "stolen", t0)
	if err := repo.Save(ctx, other); !errors.Is(err, outbound.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
	if _, ok, _ := repo.FindByID(ctx, "i3"); ok {
		t.Error("failed save must roll back the item row too")
	}
	got, _, _ := repo.FindByID(ctx, "i1")
	if occ := got.Occurrences(); len(occ) != 1 || occ[0].Context() != "x" {
		t.Errorf("occurrence of another aggregate was modified: %+v", occ)
	}
}

func TestDeleteCascades(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)
	ctx := context.Background()

	item := newItem(t, "i1", "robust", "en", "fa")
	addOcc(t, item, "o1", "x", t0)
	if err := repo.Save(ctx, item); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, "i1"); err != nil {
		t.Fatal(err)
	}
	if count(t, db, "vocabulary_items") != 0 || count(t, db, "occurrences") != 0 {
		t.Error("expected item and occurrences to be removed")
	}
	if err := repo.Delete(ctx, "i1"); err != nil {
		t.Errorf("deleting a missing item should not fail: %v", err)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	db, _ := openTemp(t)

	_, err := db.Exec(`INSERT INTO occurrences (id, vocabulary_item_id, context, source_url, captured_at) VALUES ('o','missing','','',0)`)
	if err == nil {
		t.Fatal("expected foreign key violation")
	}
}

func TestList(t *testing.T) {
	db, _ := openTemp(t)
	repo := NewVocabularyRepository(db)
	ctx := context.Background()

	mk := func(id, term string, src, tgt vocab.LanguageCode, age time.Duration) {
		item, err := vocab.NewVocabularyItem(vocab.ID(id), term, src, tgt, t0.Add(age))
		if err != nil {
			t.Fatal(err)
		}
		addOcc(t, item, "o-"+id, "ctx "+id, t0.Add(age))
		if err := repo.Save(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	mk("a", "robust", "en", "fa", 0)
	mk("b", "robustness", "en", "fa", time.Hour)
	mk("c", "ephemeral", "en", "fa", 2*time.Hour)
	mk("d", "robust", "fa", "en", 3*time.Hour)
	mk("e", "100%_sure", "en", "fa", 4*time.Hour)

	ids := func(items []*vocab.VocabularyItem) string {
		var s []string
		for _, i := range items {
			s = append(s, string(i.ID()))
			if len(i.Occurrences()) != 1 {
				t.Errorf("item %s: expected occurrences to be loaded", i.ID())
			}
		}
		return strings.Join(s, ",")
	}
	fa, en := vocab.LanguageCodePersian, vocab.LanguageCodeEnglish

	cases := []struct {
		name   string
		filter outbound.VocabItemFilter
		want   string
	}{
		{"all newest first", outbound.VocabItemFilter{}, "e,d,c,b,a"},
		{"term", outbound.VocabItemFilter{Term: "robust"}, "d,b,a"},
		{"literal wildcard", outbound.VocabItemFilter{Term: "%_"}, "e"},
		{"term + source", outbound.VocabItemFilter{Term: "robust", SourceLanguage: &en}, "b,a"},
		{"target", outbound.VocabItemFilter{TargetLanguage: &en}, "d"},
		{"source fa", outbound.VocabItemFilter{SourceLanguage: &fa}, "d"},
		{"limit", outbound.VocabItemFilter{Limit: 2}, "e,d"},
		{"limit+offset", outbound.VocabItemFilter{Limit: 2, Offset: 2}, "c,b"},
		{"offset only", outbound.VocabItemFilter{Offset: 4}, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.List(ctx, tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if s := ids(got); s != tc.want {
				t.Errorf("want %q, got %q", tc.want, s)
			}
		})
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	db, path := openTemp(t)
	ctx := context.Background()

	item := newItem(t, "i1", "robust", "en", "fa")
	addOcc(t, item, "o1", "x", t0)
	if err := NewVocabularyRepository(db).Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	got, ok, err := NewVocabularyRepository(db2).FindByID(ctx, "i1")
	if err != nil || !ok || len(got.Occurrences()) != 1 {
		t.Fatalf("data lost across reopen: ok=%v err=%v", ok, err)
	}
}

func TestMigrations(t *testing.T) {
	db, path := openTemp(t)
	ctx := context.Background()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != len(migrations) {
		t.Fatalf("expected version %d, got %d (err %v)", len(migrations), version, err)
	}

	// Idempotent.
	if err := Migrate(ctx, db); err != nil {
		t.Errorf("re-running migrations failed: %v", err)
	}

	// Databases from a newer app version are refused.
	if _, err := db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(ctx, path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("expected newer-schema error, got %v", err)
	}
}
