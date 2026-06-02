package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// InsertResults bulk-inserts per-path results for a scan using a single COPY.
func (s *Store) InsertResults(ctx context.Context, scanID string, rows []ResultRow) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := s.pool.CopyFrom(ctx,
		pgx.Identifier{"scan_results"},
		[]string{"scan_id", "url", "status_code", "status", "error", "source"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			r := rows[i]
			var code any
			if r.StatusCode != 0 {
				code = r.StatusCode
			}
			return []any{scanID, r.URL, code, r.Status, nullify(r.Error), r.Source}, nil
		}),
	)
	return err
}

// ListResults returns all stored results for a scan.
func (s *Store) ListResults(ctx context.Context, scanID string) ([]ResultRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT url, COALESCE(status_code, 0), COALESCE(status, ''),
		       COALESCE(error, ''), source
		FROM scan_results WHERE scan_id = $1 ORDER BY id`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ResultRow
	for rows.Next() {
		var r ResultRow
		if err := rows.Scan(&r.URL, &r.StatusCode, &r.Status, &r.Error, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// nullify converts an empty string to nil so it stores as SQL NULL.
func nullify(s string) any {
	if s == "" {
		return nil
	}
	return s
}
