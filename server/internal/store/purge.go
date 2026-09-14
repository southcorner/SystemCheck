package store

import (
	"context"
	"time"
)

// Cutoffs holds the oldest timestamp to keep for each data class. A zero time
// means "keep everything" (retention disabled for that class).
type Cutoffs struct {
	Screenshots time.Time
	Activity    time.Time
	Security    time.Time
	Audit       time.Time
}

// RetentionCutoffs computes per-class cutoff timestamps from day windows. A
// non-positive window disables purge for that class (zero time).
func RetentionCutoffs(now time.Time, screenshotsDays, activityDays, securityDays, auditDays int) Cutoffs {
	cut := func(days int) time.Time {
		if days <= 0 {
			return time.Time{}
		}
		return now.AddDate(0, 0, -days)
	}
	return Cutoffs{
		Screenshots: cut(screenshotsDays),
		Activity:    cut(activityDays),
		Security:    cut(securityDays),
		Audit:       cut(auditDays),
	}
}

var activityKinds = []string{"foreground", "dns", "netflow", "download"}
var securityKinds = []string{"usb", "printjob", "install", "posture", "seclog"}

// PurgeResult reports how many rows/blobs each class removed.
type PurgeResult struct {
	Events      int64
	Screenshots int64
	AppUsage    int64
	Audit       int64
}

// Purge deletes data older than the cutoffs. Screenshot blobs are removed via
// delBlob before their metadata rows. It is safe to call repeatedly.
func (s *Store) Purge(ctx context.Context, c Cutoffs, delBlob func(key string) error) (PurgeResult, error) {
	var res PurgeResult

	if !c.Activity.IsZero() {
		if err := s.purgeEventKinds(ctx, activityKinds, c.Activity, &res.Events); err != nil {
			return res, err
		}
		tag, err := s.Pool.Exec(ctx, `DELETE FROM app_usage WHERE day < $1::date`, c.Activity)
		if err != nil {
			return res, err
		}
		res.AppUsage = tag.RowsAffected()
	}

	if !c.Security.IsZero() {
		if err := s.purgeEventKinds(ctx, securityKinds, c.Security, &res.Events); err != nil {
			return res, err
		}
	}

	if !c.Screenshots.IsZero() {
		n, err := s.purgeScreenshots(ctx, c.Screenshots, delBlob)
		if err != nil {
			return res, err
		}
		res.Screenshots = n
		// Also drop the screenshot metadata events.
		if err := s.purgeEventKinds(ctx, []string{"screenshot"}, c.Screenshots, &res.Events); err != nil {
			return res, err
		}
	}

	if !c.Audit.IsZero() {
		tag, err := s.Pool.Exec(ctx, `DELETE FROM audit_log WHERE ts < $1`, c.Audit)
		if err != nil {
			return res, err
		}
		res.Audit = tag.RowsAffected()
	}

	return res, nil
}

func (s *Store) purgeEventKinds(ctx context.Context, kinds []string, before time.Time, counter *int64) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM events WHERE kind = ANY($1) AND ts < $2`, kinds, before)
	if err != nil {
		return err
	}
	*counter += tag.RowsAffected()
	return nil
}

func (s *Store) purgeScreenshots(ctx context.Context, before time.Time, delBlob func(key string) error) (int64, error) {
	rows, err := s.Pool.Query(ctx, `SELECT object_key FROM screenshots WHERE ts < $1`, before)
	if err != nil {
		return 0, err
	}
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if delBlob != nil {
		for _, k := range keys {
			_ = delBlob(k) // best-effort; row deletion below still proceeds
		}
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM screenshots WHERE ts < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
