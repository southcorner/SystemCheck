//go:build windows

package seclog

import (
	"context"
	"encoding/xml"
	"os/exec"
	"strconv"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// query selects logon (4624), failed logon (4625) and special-privilege (4672)
// events, newest first, bounded.
const query = `*[System[(EventID=4624 or EventID=4625 or EventID=4672)]]`

type secEvent struct {
	System struct {
		EventID       int   `xml:"EventID"`
		EventRecordID int64 `xml:"EventRecordID"`
		TimeCreated   struct {
			SystemTime string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
	} `xml:"System"`
	EventData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventData"`
}

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var lastRecord int64
	// Seed lastRecord silently so we only emit new events after startup.
	if evs := c.poll(ctx); len(evs) > 0 {
		lastRecord = evs[0].System.EventRecordID
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		evs := c.poll(ctx)
		newMax := lastRecord
		for _, e := range evs {
			if e.System.EventRecordID <= lastRecord {
				continue
			}
			if e.System.EventRecordID > newMax {
				newMax = e.System.EventRecordID
			}
			emit(toEvent(e))
		}
		lastRecord = newMax
	}
}

func (c *Collector) poll(ctx context.Context) []secEvent {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "wevtutil", "qe", "Security",
		"/q:"+query, "/f:xml", "/c:50", "/rd:true").Output()
	if err != nil {
		return nil
	}
	// wevtutil emits a bare sequence of <Event> elements; wrap for parsing.
	wrapped := append([]byte("<Events>"), out...)
	wrapped = append(wrapped, []byte("</Events>")...)
	var doc struct {
		Events []secEvent `xml:"Event"`
	}
	if err := xml.Unmarshal(wrapped, &doc); err != nil {
		return nil
	}
	return doc.Events
}

func toEvent(e secEvent) wire.Event {
	data := map[string]string{}
	for _, d := range e.EventData.Data {
		data[d.Name] = d.Value
	}
	kind := "other"
	switch e.System.EventID {
	case 4624:
		kind = "logon"
	case 4625:
		kind = "failed_logon"
	case 4672:
		kind = "priv"
	}
	account := data["TargetUserName"]
	if account == "" {
		account = data["SubjectUserName"]
	}
	ts := time.Now().UTC()
	if t, err := time.Parse(time.RFC3339Nano, e.System.TimeCreated.SystemTime); err == nil {
		ts = t.UTC()
	}
	return wire.Event{
		Kind: "seclog",
		TS:   ts,
		Data: map[string]interface{}{
			"event_id":   e.System.EventID,
			"kind":       kind,
			"account":    account,
			"logon_type": data["LogonType"],
			"source_ip":  data["IpAddress"],
			"record_id":  strconv.FormatInt(e.System.EventRecordID, 10),
		},
	}
}
