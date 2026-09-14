package store

import (
	"context"
	"time"
)

// DomainRow is an accessed-domain rollup.
type DomainRow struct {
	Domain   string    `json:"domain"`
	Count    int64     `json:"count"`
	LastSeen time.Time `json:"last_seen"`
}

// Domains aggregates DNS events for a machine within a window.
func (s *Store) Domains(ctx context.Context, machineID string, from, to time.Time, limit int) ([]DomainRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT data->>'domain' AS domain, count(*) AS n, max(ts) AS last_seen
		   FROM events
		  WHERE machine_id=$1 AND kind='dns' AND ts BETWEEN $2 AND $3
		    AND data->>'domain' IS NOT NULL
		  GROUP BY data->>'domain'
		  ORDER BY n DESC
		  LIMIT $4`, machineID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DomainRow
	for rows.Next() {
		var r DomainRow
		if err := rows.Scan(&r.Domain, &r.Count, &r.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TransferRow is a per-process byte total.
type TransferRow struct {
	Process   string `json:"process"`
	SentBytes int64  `json:"sent_bytes"`
	RecvBytes int64  `json:"recv_bytes"`
}

// Transfers aggregates netflow events for a machine within a window.
func (s *Store) Transfers(ctx context.Context, machineID string, from, to time.Time) ([]TransferRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT coalesce(data->>'process','') AS process,
		        sum(coalesce((data->>'sent_bytes')::bigint,0)) AS sent,
		        sum(coalesce((data->>'recv_bytes')::bigint,0)) AS recv
		   FROM events
		  WHERE machine_id=$1 AND kind='netflow' AND ts BETWEEN $2 AND $3
		  GROUP BY data->>'process'
		  ORDER BY sent DESC`, machineID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TransferRow
	for rows.Next() {
		var r TransferRow
		if err := rows.Scan(&r.Process, &r.SentBytes, &r.RecvBytes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DownloadRow is a single download/upload file record.
type DownloadRow struct {
	TS     time.Time `json:"ts"`
	Name   string    `json:"name"`
	Path   string    `json:"path"`
	Size   int64     `json:"size"`
	URL    string    `json:"url"`
	Source string    `json:"source"`
}

// Downloads lists download events for a machine within a window.
func (s *Store) Downloads(ctx context.Context, machineID string, from, to time.Time, limit int) ([]DownloadRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT ts,
		        coalesce(data->>'name','') AS name,
		        coalesce(data->>'path','') AS path,
		        coalesce((data->>'size')::bigint,0) AS size,
		        coalesce(data->>'url','') AS url,
		        coalesce(data->>'source','') AS source
		   FROM events
		  WHERE machine_id=$1 AND kind='download' AND ts BETWEEN $2 AND $3
		  ORDER BY ts DESC
		  LIMIT $4`, machineID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DownloadRow
	for rows.Next() {
		var r DownloadRow
		if err := rows.Scan(&r.TS, &r.Name, &r.Path, &r.Size, &r.URL, &r.Source); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
