//go:build windows

package browsing

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// chromeEpochOffsetMicros converts Chrome/WebKit time (since 1601) to Unix.
const chromeEpochOffsetMicros = 11644473600000000

// maxPerScan bounds how many visits one scan emits, so a large history is
// imported gradually rather than in one burst.
const maxPerScan = 3000

// initialLookback is how far back the first scan of a DB imports.
const initialLookback = 7 * 24 * time.Hour

type histSrc struct {
	browser string
	profile string
	path    string
	firefox bool
}

func scanAll(emit collectors.Emit) {
	wm := loadWatermarks()
	srcs := append(chromiumHistories(), firefoxHistories()...)
	for _, s := range srcs {
		if s.firefox {
			scanFirefox(s, emit, wm)
		} else {
			scanChromium(s, emit, wm)
		}
	}
	saveWatermarks(wm)
}

// chromiumHistories finds History DBs for all Chromium browsers and profiles.
func chromiumHistories() []histSrc {
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	var out []histSrc

	// "User Data"-style browsers with multiple profiles.
	userData := []histSrc{
		{browser: "chrome", path: filepath.Join(local, "Google", "Chrome", "User Data")},
		{browser: "edge", path: filepath.Join(local, "Microsoft", "Edge", "User Data")},
		{browser: "brave", path: filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data")},
		{browser: "vivaldi", path: filepath.Join(local, "Vivaldi", "User Data")},
	}
	for _, b := range userData {
		entries, err := os.ReadDir(b.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name != "Default" && !strings.HasPrefix(name, "Profile ") {
				continue
			}
			h := filepath.Join(b.path, name, "History")
			if fileExists(h) {
				out = append(out, histSrc{browser: b.browser, profile: name, path: h})
			}
		}
	}

	// Opera stores a single profile directly under its data dir (no "User Data").
	opera := []histSrc{
		{browser: "opera", path: filepath.Join(roaming, "Opera Software", "Opera Stable")},
		{browser: "opera-gx", path: filepath.Join(roaming, "Opera Software", "Opera GX Stable")},
	}
	for _, o := range opera {
		h := filepath.Join(o.path, "History")
		if fileExists(h) {
			out = append(out, histSrc{browser: o.browser, profile: "Default", path: h})
		}
	}
	return out
}

func firefoxHistories() []histSrc {
	base := filepath.Join(os.Getenv("APPDATA"), "Mozilla", "Firefox", "Profiles")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []histSrc
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		h := filepath.Join(base, e.Name(), "places.sqlite")
		if fileExists(h) {
			out = append(out, histSrc{browser: "firefox", profile: e.Name(), path: h, firefox: true})
		}
	}
	return out
}

func scanChromium(s histSrc, emit collectors.Emit, wm map[string]int64) {
	tmp, err := copyToTemp(s.path)
	if err != nil {
		return
	}
	defer os.Remove(tmp)
	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro&immutable=1")
	if err != nil {
		return
	}
	defer db.Close()

	mark := wm[s.path]
	if mark == 0 {
		mark = time.Now().Add(-initialLookback).UnixMicro() + chromeEpochOffsetMicros
	}
	rows, err := db.Query(
		`SELECT u.url, COALESCE(u.title,''), v.visit_time
		   FROM visits v JOIN urls u ON u.id = v.url
		  WHERE v.visit_time > ? ORDER BY v.visit_time ASC LIMIT ?`, mark, maxPerScan)
	if err != nil {
		return
	}
	defer rows.Close()
	newMark := mark
	for rows.Next() {
		var u, title string
		var vt int64
		if rows.Scan(&u, &title, &vt) != nil {
			continue
		}
		if vt > newMark {
			newMark = vt
		}
		emitVisit(emit, u, title, s.browser, s.profile, time.UnixMicro(vt-chromeEpochOffsetMicros).UTC())
	}
	wm[s.path] = newMark
}

func scanFirefox(s histSrc, emit collectors.Emit, wm map[string]int64) {
	tmp, err := copyToTemp(s.path)
	if err != nil {
		return
	}
	defer os.Remove(tmp)
	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro&immutable=1")
	if err != nil {
		return
	}
	defer db.Close()

	mark := wm[s.path]
	if mark == 0 {
		mark = time.Now().Add(-initialLookback).UnixMicro()
	}
	rows, err := db.Query(
		`SELECT p.url, COALESCE(p.title,''), h.visit_date
		   FROM moz_historyvisits h JOIN moz_places p ON p.id = h.place
		  WHERE h.visit_date > ? ORDER BY h.visit_date ASC LIMIT ?`, mark, maxPerScan)
	if err != nil {
		return
	}
	defer rows.Close()
	newMark := mark
	for rows.Next() {
		var u, title string
		var vt int64
		if rows.Scan(&u, &title, &vt) != nil {
			continue
		}
		if vt > newMark {
			newMark = vt
		}
		emitVisit(emit, u, title, s.browser, s.profile, time.UnixMicro(vt).UTC())
	}
	wm[s.path] = newMark
}

func emitVisit(emit collectors.Emit, u, title, browser, profile string, ts time.Time) {
	if u == "" || !strings.HasPrefix(u, "http") {
		return // skip non-web (about:, file:, chrome:) entries
	}
	emit(wire.Event{
		Kind: "visit",
		TS:   ts,
		Data: map[string]interface{}{
			"url":     u,
			"title":   title,
			"domain":  domainOf(u),
			"browser": browser,
			"profile": profile,
		},
	})
}

func domainOf(raw string) string {
	if p, err := url.Parse(raw); err == nil {
		return p.Hostname()
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// copyToTemp copies the (browser-locked) DB so it can be read while the browser
// is running.
func copyToTemp(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp("", "sc-hist-*.db")
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

// --- watermark persistence (so restarts don't re-import history) ---

func stateFile() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "SystemCheck", "spool", "browsing-state.json")
}

func loadWatermarks() map[string]int64 {
	m := map[string]int64{}
	if b, err := os.ReadFile(stateFile()); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func saveWatermarks(m map[string]int64) {
	if b, err := json.Marshal(m); err == nil {
		tmp := stateFile() + ".tmp"
		if os.WriteFile(tmp, b, 0o644) == nil {
			_ = os.Rename(tmp, stateFile())
		}
	}
}
