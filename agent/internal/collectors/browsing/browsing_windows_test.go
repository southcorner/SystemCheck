//go:build windows

package browsing

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

func TestScanChromium(t *testing.T) {
	dir := t.TempDir()
	dbp := filepath.Join(dir, "History")

	db, err := sql.Open("sqlite", "file:"+dbp)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	mustExec(t, db, `CREATE TABLE urls(id INTEGER PRIMARY KEY, url TEXT, title TEXT)`)
	mustExec(t, db, `CREATE TABLE visits(id INTEGER PRIMARY KEY, url INTEGER, visit_time INTEGER)`)
	mustExec(t, db, `INSERT INTO urls(id,url,title) VALUES(1,'https://mail.google.com/mail/u/0/#inbox','Inbox (2)')`)
	vt := time.Now().UnixMicro() + chromeEpochOffsetMicros
	mustExec(t, db, `INSERT INTO visits(id,url,visit_time) VALUES(1,1,?)`, vt)
	db.Close()

	var got []wire.Event
	scanChromium(histSrc{browser: "chrome", profile: "Default", path: dbp},
		func(e wire.Event) { got = append(got, e) }, map[string]int64{})

	if len(got) != 1 {
		t.Fatalf("emitted %d events, want 1", len(got))
	}
	e := got[0]
	if e.Kind != "visit" || e.Data["domain"] != "mail.google.com" || e.Data["browser"] != "chrome" {
		t.Fatalf("unexpected event: %+v", e.Data)
	}
	if e.Data["title"] != "Inbox (2)" {
		t.Fatalf("title = %v", e.Data["title"])
	}
}

func TestScanChromiumWatermarkSkipsOld(t *testing.T) {
	dir := t.TempDir()
	dbp := filepath.Join(dir, "History")
	db, _ := sql.Open("sqlite", "file:"+dbp)
	mustExec(t, db, `CREATE TABLE urls(id INTEGER PRIMARY KEY, url TEXT, title TEXT)`)
	mustExec(t, db, `CREATE TABLE visits(id INTEGER PRIMARY KEY, url INTEGER, visit_time INTEGER)`)
	mustExec(t, db, `INSERT INTO urls(id,url,title) VALUES(1,'https://example.com/','x')`)
	vt := time.Now().UnixMicro() + chromeEpochOffsetMicros
	mustExec(t, db, `INSERT INTO visits(id,url,visit_time) VALUES(1,1,?)`, vt)
	db.Close()

	// Watermark already at/after this visit -> nothing new.
	wm := map[string]int64{dbp: vt}
	var n int
	scanChromium(histSrc{browser: "chrome", profile: "Default", path: dbp},
		func(wire.Event) { n++ }, wm)
	if n != 0 {
		t.Fatalf("emitted %d, want 0 (watermark should skip)", n)
	}
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}
