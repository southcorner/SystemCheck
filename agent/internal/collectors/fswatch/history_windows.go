//go:build windows

package fswatch

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// chromeEpochOffsetMicros is the difference between the Chrome/WebKit epoch
// (1601-01-01) and the Unix epoch, in microseconds.
const chromeEpochOffsetMicros = 11644473600000000

// lastSeen tracks the newest download start_time already emitted per History DB,
// so periodic scans don't re-report the same rows.
var (
	seenMu   sync.Mutex
	lastSeen = map[string]int64{}
)

// scanBrowserHistory reads Chrome and Edge download history and emits any new
// downloads as "download" events with source "browser".
func scanBrowserHistory(emit collectors.Emit) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return
	}
	dbs := []string{
		filepath.Join(local, "Google", "Chrome", "User Data", "Default", "History"),
		filepath.Join(local, "Microsoft", "Edge", "User Data", "Default", "History"),
	}
	for _, db := range dbs {
		if _, err := os.Stat(db); err != nil {
			continue
		}
		scanOne(db, emit)
	}
}

func scanOne(dbPath string, emit collectors.Emit) {
	// The live History DB is locked while the browser runs; copy it first.
	tmp, err := copyToTemp(dbPath)
	if err != nil {
		return
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro&immutable=1")
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query(`SELECT target_path, total_bytes, tab_url, start_time FROM downloads ORDER BY start_time ASC`)
	if err != nil {
		return
	}
	defer rows.Close()

	seenMu.Lock()
	watermark := lastSeen[dbPath]
	seenMu.Unlock()
	newWatermark := watermark

	for rows.Next() {
		var targetPath, tabURL string
		var totalBytes, startTime int64
		if err := rows.Scan(&targetPath, &totalBytes, &tabURL, &startTime); err != nil {
			continue
		}
		if startTime <= watermark {
			continue
		}
		if startTime > newWatermark {
			newWatermark = startTime
		}
		ts := time.UnixMicro(startTime - chromeEpochOffsetMicros).UTC()
		emit(wire.Event{
			Kind: "download",
			TS:   ts,
			Data: map[string]interface{}{
				"name":   filepath.Base(targetPath),
				"path":   targetPath,
				"size":   totalBytes,
				"url":    tabURL,
				"source": "browser",
			},
		})
	}

	seenMu.Lock()
	lastSeen[dbPath] = newWatermark
	seenMu.Unlock()
}

func copyToTemp(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp("", "sc-history-*.db")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(out.Name())
		return "", err
	}
	out.Close()
	return out.Name(), nil
}
