package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// VocabularyRepository persists the VocabularyItem aggregate in SQLite.
type VocabularyRepository struct {
	db *sql.DB
}

var _ outbound.VocabularyRepository = (*VocabularyRepository)(nil)

func NewVocabularyRepository(db *sql.DB) *VocabularyRepository {
	return &VocabularyRepository{db: db}
}

const itemColumns = "id, term, source_language, target_language, created_at"

func (r *VocabularyRepository) FindByID(ctx context.Context, id vocab.ID) (*vocab.VocabularyItem, bool, error) {
	return r.findOne(ctx, "id = ?", string(id))
}

func (r *VocabularyRepository) FindByKey(ctx context.Context, key vocab.VocabularyKey) (*vocab.VocabularyItem, bool, error) {
	return r.findOne(ctx,
		"normalized_term = ? AND source_language = ? AND target_language = ?",
		key.NormalizedTerm, string(key.SourceLanguage), string(key.TargetLanguage))
}

func (r *VocabularyRepository) findOne(ctx context.Context, where string, args ...any) (*vocab.VocabularyItem, bool, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+itemColumns+" FROM vocabulary_items WHERE "+where, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query vocabulary item: %w", err)
	}
	records, err := scanItems(rows)
	if err != nil {
		return nil, false, err
	}
	if len(records) == 0 {
		return nil, false, nil
	}

	items, err := r.hydrate(ctx, records)
	if err != nil {
		return nil, false, err
	}
	return items[0], true, nil
}

func (r *VocabularyRepository) List(ctx context.Context, f outbound.VocabItemFilter) ([]*vocab.VocabularyItem, error) {
	var (
		conds []string
		args  []any
	)
	if f.Term != "" {
		conds = append(conds, `normalized_term LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(f.Term)+"%")
	}
	if f.SourceLanguage != nil {
		conds = append(conds, "source_language = ?")
		args = append(args, string(*f.SourceLanguage))
	}
	if f.TargetLanguage != nil {
		conds = append(conds, "target_language = ?")
		args = append(args, string(*f.TargetLanguage))
	}

	query := "SELECT " + itemColumns + " FROM vocabulary_items"
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at DESC, id"

	limit := -1 // SQLite: negative means no limit
	if f.Limit > 0 {
		limit = f.Limit
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list vocabulary items: %w", err)
	}
	records, err := scanItems(rows)
	if err != nil {
		return nil, err
	}

	return r.hydrate(ctx, records)
}

// Save inserts or updates the aggregate and synchronises its occurrences in a
// single transaction.
func (r *VocabularyRepository) Save(ctx context.Context, item *vocab.VocabularyItem) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO vocabulary_items (id, term, normalized_term, source_language, target_language, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			term = excluded.term,
			normalized_term = excluded.normalized_term,
			source_language = excluded.source_language,
			target_language = excluded.target_language`,
		string(item.ID()), item.Term(), item.NormalizedTerm(),
		string(item.SourceLanguage()), string(item.TargetLanguage()), item.CreatedAt().UnixNano())
	if err != nil {
		return mapSaveError("save vocabulary item", err)
	}

	occurrences := item.Occurrences()
	keep := make([]any, 0, len(occurrences)+1)
	keep = append(keep, string(item.ID()))

	for _, o := range occurrences {
		var res sql.Result
		res, err = tx.ExecContext(ctx, `
			INSERT INTO occurrences (id, vocabulary_item_id, context, source_url, captured_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				context = excluded.context,
				source_url = excluded.source_url,
				captured_at = excluded.captured_at
			WHERE vocabulary_item_id = excluded.vocabulary_item_id`,
			string(o.ID()), string(o.VocabularyItemID()), o.Context(), o.SourceURL(), o.CapturedAt().UnixNano())
		if err != nil {
			return mapSaveError("save occurrence", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// The ID is already used by an occurrence of another aggregate.
			err = fmt.Errorf("save occurrence %s: %w", o.ID(), outbound.ErrConflict)
			return err
		}
		keep = append(keep, string(o.ID()))
	}

	// Remove occurrences no longer in the aggregate.
	del := "DELETE FROM occurrences WHERE vocabulary_item_id = ?"
	if len(occurrences) > 0 {
		del += " AND id NOT IN (" + placeholders(len(occurrences)) + ")"
	}
	if _, err = tx.ExecContext(ctx, del, keep...); err != nil {
		return fmt.Errorf("remove stale occurrences: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *VocabularyRepository) Delete(ctx context.Context, id vocab.ID) error {
	// Occurrences are removed by ON DELETE CASCADE.
	if _, err := r.db.ExecContext(ctx, "DELETE FROM vocabulary_items WHERE id = ?", string(id)); err != nil {
		return fmt.Errorf("delete vocabulary item: %w", err)
	}
	return nil
}

type itemRecord struct {
	id, term       string
	source, target string
	createdAt      int64
}

func scanItems(rows *sql.Rows) ([]itemRecord, error) {
	defer rows.Close()

	var out []itemRecord
	for rows.Next() {
		var rec itemRecord
		if err := rows.Scan(&rec.id, &rec.term, &rec.source, &rec.target, &rec.createdAt); err != nil {
			return nil, fmt.Errorf("scan vocabulary item: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read vocabulary items: %w", err)
	}
	return out, nil
}

// hydrate loads occurrences for the records and rebuilds the aggregates,
// preserving record order. Records are fully read before this is called, so
// the single connection is free.
func (r *VocabularyRepository) hydrate(ctx context.Context, records []itemRecord) ([]*vocab.VocabularyItem, error) {
	byItem := make(map[string][]vocab.Occurrence, len(records))

	const chunk = 500
	for start := 0; start < len(records); start += chunk {
		end := min(start+chunk, len(records))
		args := make([]any, 0, end-start)
		for _, rec := range records[start:end] {
			args = append(args, rec.id)
		}

		rows, err := r.db.QueryContext(ctx,
			"SELECT id, vocabulary_item_id, context, source_url, captured_at FROM occurrences WHERE vocabulary_item_id IN ("+
				placeholders(len(args))+") ORDER BY captured_at, rowid", args...)
		if err != nil {
			return nil, fmt.Errorf("query occurrences: %w", err)
		}

		err = func() error {
			defer rows.Close()
			for rows.Next() {
				var id, itemID, ctxText, url string
				var capturedAt int64
				if err := rows.Scan(&id, &itemID, &ctxText, &url, &capturedAt); err != nil {
					return fmt.Errorf("scan occurrence: %w", err)
				}
				o, err := vocab.NewOccurrence(vocab.ID(id), vocab.ID(itemID), ctxText, url, time.Unix(0, capturedAt))
				if err != nil {
					return fmt.Errorf("restore occurrence %s: %w", id, err)
				}
				byItem[itemID] = append(byItem[itemID], o)
			}
			return rows.Err()
		}()
		if err != nil {
			return nil, err
		}
	}

	items := make([]*vocab.VocabularyItem, 0, len(records))
	for _, rec := range records {
		item, err := vocab.RestoreVocabularyItem(
			vocab.ID(rec.id), rec.term,
			vocab.LanguageCode(rec.source), vocab.LanguageCode(rec.target),
			time.Unix(0, rec.createdAt), byItem[rec.id])
		if err != nil {
			return nil, fmt.Errorf("restore vocabulary item %s: %w", rec.id, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func mapSaveError(msg string, err error) error {
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
			return fmt.Errorf("%s: %w: %w", msg, outbound.ErrConflict, err)
		}
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
