// Package alert evaluates detection rules against incoming events and records
// alerts. Evaluate is pure (no I/O) so it is unit-testable; Process wires it to
// the store during ingestion.
package alert

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/southcorner/systemcheck/server/internal/model"
	"github.com/southcorner/systemcheck/server/internal/store"
)

// Match is an alert produced by a rule for one event.
type Match struct {
	RuleID   string
	Severity string
	Message  string
	Data     map[string]interface{}
}

// Evaluate returns the matches produced by rules for a single event.
func Evaluate(rules []store.AlertRule, e model.Event) []Match {
	var matches []Match
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Kind {
		case "domain_blocklist":
			if e.Kind != "dns" {
				continue
			}
			domain := strings.ToLower(strings.TrimSuffix(str(e.Data["domain"]), "."))
			if domain == "" {
				continue
			}
			for _, pat := range strList(r.Params["domains"]) {
				if domainMatch(pat, domain) {
					matches = append(matches, Match{
						RuleID: r.ID, Severity: "warning",
						Message: fmt.Sprintf("Blocked domain accessed: %s", domain),
						Data:    map[string]interface{}{"domain": domain, "process": str(e.Data["process"]), "pattern": pat},
					})
					break
				}
			}
		case "large_upload":
			if e.Kind != "netflow" {
				continue
			}
			sent := toInt64(e.Data["sent_bytes"])
			threshold := toInt64(r.Params["threshold_bytes"])
			if threshold > 0 && sent > threshold {
				matches = append(matches, Match{
					RuleID: r.ID, Severity: "warning",
					Message: fmt.Sprintf("Large upload: %s sent %d bytes", str(e.Data["process"]), sent),
					Data:    map[string]interface{}{"process": str(e.Data["process"]), "sent_bytes": sent, "threshold_bytes": threshold},
				})
			}
		case "app_blocklist":
			if e.Kind != "foreground" {
				continue
			}
			proc := str(e.Data["process"])
			for _, p := range strList(r.Params["processes"]) {
				if strings.EqualFold(p, proc) {
					matches = append(matches, Match{
						RuleID: r.ID, Severity: "info",
						Message: fmt.Sprintf("Blocked application used: %s", proc),
						Data:    map[string]interface{}{"process": proc},
					})
					break
				}
			}
		case "dlp_keyword":
			if e.Kind != "download" {
				continue
			}
			hay := strings.ToLower(str(e.Data["name"]) + " " + str(e.Data["path"]) + " " + str(e.Data["url"]))
			for _, pat := range strList(r.Params["patterns"]) {
				re, err := regexp.Compile("(?i)" + pat)
				if err != nil {
					continue
				}
				if re.MatchString(hay) {
					matches = append(matches, Match{
						RuleID: r.ID, Severity: "warning",
						Message: fmt.Sprintf("DLP match on file: %s", str(e.Data["name"])),
						Data:    map[string]interface{}{"name": str(e.Data["name"]), "path": str(e.Data["path"]), "pattern": pat},
					})
					break
				}
			}
		case "usb_insert":
			if e.Kind != "usb" || str(e.Data["action"]) != "insert" {
				continue
			}
			matches = append(matches, Match{
				RuleID: r.ID, Severity: "warning",
				Message: fmt.Sprintf("Removable media inserted: %s (%s)", str(e.Data["drive"]), str(e.Data["label"])),
				Data:    map[string]interface{}{"drive": str(e.Data["drive"]), "label": str(e.Data["label"]), "serial": str(e.Data["serial"])},
			})
		case "failed_logon":
			if e.Kind != "seclog" || str(e.Data["kind"]) != "failed_logon" {
				continue
			}
			matches = append(matches, Match{
				RuleID: r.ID, Severity: "warning",
				Message: fmt.Sprintf("Failed sign-in: account %s (type %s)", str(e.Data["account"]), str(e.Data["logon_type"])),
				Data:    map[string]interface{}{"account": str(e.Data["account"]), "source_ip": str(e.Data["source_ip"]), "logon_type": str(e.Data["logon_type"])},
			})
		case "new_install":
			if e.Kind != "install" {
				continue
			}
			matches = append(matches, Match{
				RuleID: r.ID, Severity: "info",
				Message: fmt.Sprintf("New software installed: %s %s", str(e.Data["name"]), str(e.Data["version"])),
				Data:    map[string]interface{}{"name": str(e.Data["name"]), "version": str(e.Data["version"])},
			})
		}
	}
	return matches
}

// Process loads enabled rules once and records alerts for a batch of events. For
// each recorded alert it invokes notify (if non-nil), e.g. to forward to a
// SIEM/webhook. Notification failures do not fail Process.
func Process(ctx context.Context, st *store.Store, machineID string, events []model.Event, notify func(Match)) error {
	rules, err := st.ListAlertRules(ctx, false)
	if err != nil || len(rules) == 0 {
		return err
	}
	for _, e := range events {
		for _, m := range Evaluate(rules, e) {
			if err := st.CreateAlert(ctx, m.RuleID, machineID, m.Severity, m.Message, m.Data); err != nil {
				return err
			}
			if notify != nil {
				notify(m)
			}
		}
	}
	return nil
}

// domainMatch matches a domain against a pattern supporting exact, glob
// (path.Match), and "*.suffix" forms.
func domainMatch(pattern, domain string) bool {
	pattern = strings.ToLower(pattern)
	if pattern == domain {
		return true
	}
	if ok, _ := path.Match(pattern, domain); ok {
		return true
	}
	if strings.HasPrefix(pattern, "*.") && strings.HasSuffix(domain, pattern[1:]) {
		return true
	}
	return false
}

func str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return 0
	}
}

func strList(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
