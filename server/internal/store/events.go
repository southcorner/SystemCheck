package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/southcorner/systemcheck/server/internal/model"
)

// InsertEvents writes a batch of events for a machine and returns how many were
// accepted. Screenshot-kind events additionally create a screenshots metadata
// row. Excluded/unknown data is stored as-is (JSONB) for forward compatibility.
func (s *Store) InsertEvents(ctx context.Context, machineID string, events []model.Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, e := range events {
		ts := e.TS
		if ts.IsZero() {
			ts = time.Now()
		}
		batch.Queue(
			`INSERT INTO events(ts, machine_id, kind, data) VALUES($1,$2,$3,$4)`,
			ts, machineID, e.Kind, e.Data)

		switch e.Kind {
		case "screenshot":
			batch.Queue(
				`INSERT INTO screenshots(machine_id, ts, object_key, width, height, bytes, format)
				 VALUES($1,$2,$3,$4,$5,$6,$7)`,
				machineID, ts,
				str(e.Data["object_key"]), num(e.Data["width"]), num(e.Data["height"]),
				num(e.Data["bytes"]), str(e.Data["format"]))
		case "foreground":
			batch.Queue(
				`INSERT INTO app_usage(machine_id, day, process, active_sec)
				 VALUES($1, ($2::timestamptz)::date, $3, $4)
				 ON CONFLICT (machine_id, day, process)
				 DO UPDATE SET active_sec = app_usage.active_sec + EXCLUDED.active_sec`,
				machineID, ts, str(e.Data["process"]), num(e.Data["active_sec"]))
		}
	}
	br := s.Pool.SendBatch(ctx, batch)
	defer br.Close()
	// Drain results so errors surface.
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

// ScreenshotMeta is a screenshot metadata row.
type ScreenshotMeta struct {
	ID        string    `json:"id"`
	TS        time.Time `json:"ts"`
	ObjectKey string    `json:"object_key"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Bytes     int64     `json:"bytes"`
	Format    string    `json:"format"`
}

// ListScreenshots returns screenshot metadata for a machine within a window.
func (s *Store) ListScreenshots(ctx context.Context, machineID string, from, to time.Time, limit int) ([]ScreenshotMeta, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, ts, object_key, coalesce(width,0), coalesce(height,0), coalesce(bytes,0), coalesce(format,'')
		   FROM screenshots WHERE machine_id=$1 AND ts BETWEEN $2 AND $3
		 ORDER BY ts DESC LIMIT $4`, machineID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScreenshotMeta
	for rows.Next() {
		var m ScreenshotMeta
		if err := rows.Scan(&m.ID, &m.TS, &m.ObjectKey, &m.Width, &m.Height, &m.Bytes, &m.Format); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ScreenshotByID returns one screenshot's object key (for streaming the blob).
func (s *Store) ScreenshotByID(ctx context.Context, id string) (machineID, objectKey, format string, err error) {
	err = s.Pool.QueryRow(ctx,
		`SELECT machine_id, object_key, coalesce(format,'webp') FROM screenshots WHERE id=$1`, id,
	).Scan(&machineID, &objectKey, &format)
	if err != nil {
		err = noRows(err)
	}
	return
}

// AppUsageRow is a rolled-up per-process active-time entry.
type AppUsageRow struct {
	Process   string `json:"process"`
	ActiveSec int64  `json:"active_sec"`
}

// AppUsage returns aggregated app usage for a machine within a day range.
func (s *Store) AppUsage(ctx context.Context, machineID string, from, to time.Time) ([]AppUsageRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT process, sum(active_sec) FROM app_usage
		  WHERE machine_id=$1 AND day BETWEEN $2::date AND $3::date
		  GROUP BY process ORDER BY sum(active_sec) DESC`, machineID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppUsageRow
	for rows.Next() {
		var r AppUsageRow
		if err := rows.Scan(&r.Process, &r.ActiveSec); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EventRow is a generic stored event.
type EventRow struct {
	TS   time.Time              `json:"ts"`
	Kind string                 `json:"kind"`
	Data map[string]interface{} `json:"data"`
}

// ListEvents returns events for a machine, optionally filtered by kind.
func (s *Store) ListEvents(ctx context.Context, machineID, kind string, from, to time.Time, limit int) ([]EventRow, error) {
	q := `SELECT ts, kind, data FROM events WHERE machine_id=$1 AND ts BETWEEN $2 AND $3`
	args := []any{machineID, from, to}
	if kind != "" {
		q += ` AND kind=$4 ORDER BY ts DESC LIMIT $5`
		args = append(args, kind, limit)
	} else {
		q += ` ORDER BY ts DESC LIMIT $4`
		args = append(args, limit)
	}
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var r EventRow
		if err := rows.Scan(&r.TS, &r.Kind, &r.Data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- small coercion helpers for JSON-decoded values ---

func str(v any) any {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		return s
	}
	return nil
}

func num(v any) any {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	default:
		return nil
	}
}
