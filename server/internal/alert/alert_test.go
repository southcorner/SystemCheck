package alert

import (
	"testing"

	"github.com/southcorner/systemcheck/server/internal/model"
	"github.com/southcorner/systemcheck/server/internal/store"
)

func TestDomainBlocklist(t *testing.T) {
	rules := []store.AlertRule{{
		ID: "r1", Kind: "domain_blocklist", Enabled: true,
		Params: map[string]interface{}{"domains": []interface{}{"*.dropbox.com", "wetransfer.com"}},
	}}
	// matches wildcard suffix
	m := Evaluate(rules, model.Event{Kind: "dns", Data: map[string]interface{}{"domain": "dl.dropbox.com", "process": "chrome.exe"}})
	if len(m) != 1 {
		t.Fatalf("expected 1 match, got %d", len(m))
	}
	// exact
	m = Evaluate(rules, model.Event{Kind: "dns", Data: map[string]interface{}{"domain": "wetransfer.com"}})
	if len(m) != 1 {
		t.Fatalf("expected exact match, got %d", len(m))
	}
	// non-match
	m = Evaluate(rules, model.Event{Kind: "dns", Data: map[string]interface{}{"domain": "example.com"}})
	if len(m) != 0 {
		t.Fatalf("expected no match, got %d", len(m))
	}
	// wrong kind
	m = Evaluate(rules, model.Event{Kind: "foreground", Data: map[string]interface{}{"domain": "dl.dropbox.com"}})
	if len(m) != 0 {
		t.Fatalf("expected no match for wrong kind, got %d", len(m))
	}
}

func TestLargeUpload(t *testing.T) {
	rules := []store.AlertRule{{
		ID: "r2", Kind: "large_upload", Enabled: true,
		Params: map[string]interface{}{"threshold_bytes": float64(1000000)},
	}}
	// JSON numbers arrive as float64
	m := Evaluate(rules, model.Event{Kind: "netflow", Data: map[string]interface{}{"process": "curl.exe", "sent_bytes": float64(5000000)}})
	if len(m) != 1 {
		t.Fatalf("expected large-upload match, got %d", len(m))
	}
	m = Evaluate(rules, model.Event{Kind: "netflow", Data: map[string]interface{}{"process": "curl.exe", "sent_bytes": float64(500)}})
	if len(m) != 0 {
		t.Fatalf("expected no match under threshold, got %d", len(m))
	}
}

func TestDisabledRuleIgnored(t *testing.T) {
	rules := []store.AlertRule{{
		ID: "r3", Kind: "domain_blocklist", Enabled: false,
		Params: map[string]interface{}{"domains": []interface{}{"example.com"}},
	}}
	if m := Evaluate(rules, model.Event{Kind: "dns", Data: map[string]interface{}{"domain": "example.com"}}); len(m) != 0 {
		t.Fatalf("disabled rule should not match, got %d", len(m))
	}
}

func TestDLPAndUSB(t *testing.T) {
	rules := []store.AlertRule{
		{ID: "d1", Kind: "dlp_keyword", Enabled: true, Params: map[string]interface{}{"patterns": []interface{}{`payroll|salary|\.sql$`}}},
		{ID: "u1", Kind: "usb_insert", Enabled: true, Params: map[string]interface{}{}},
		{ID: "n1", Kind: "new_install", Enabled: true, Params: map[string]interface{}{}},
	}
	if m := Evaluate(rules, model.Event{Kind: "download", Data: map[string]interface{}{"name": "2026_payroll.xlsx"}}); len(m) != 1 {
		t.Fatalf("dlp expected 1, got %d", len(m))
	}
	if m := Evaluate(rules, model.Event{Kind: "download", Data: map[string]interface{}{"name": "cat.jpg"}}); len(m) != 0 {
		t.Fatalf("dlp expected 0, got %d", len(m))
	}
	if m := Evaluate(rules, model.Event{Kind: "usb", Data: map[string]interface{}{"action": "insert", "drive": "E:\\"}}); len(m) != 1 {
		t.Fatalf("usb insert expected 1, got %d", len(m))
	}
	if m := Evaluate(rules, model.Event{Kind: "usb", Data: map[string]interface{}{"action": "remove", "drive": "E:\\"}}); len(m) != 0 {
		t.Fatalf("usb remove expected 0, got %d", len(m))
	}
	if m := Evaluate(rules, model.Event{Kind: "install", Data: map[string]interface{}{"name": "uTorrent", "version": "3.5"}}); len(m) != 1 {
		t.Fatalf("new_install expected 1, got %d", len(m))
	}
}
