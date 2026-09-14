//go:build windows

package printjobs

import (
	"context"
	"time"

	"github.com/0xrawsec/golang-etw/etw"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/winutil"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// printedEventID is "a document was printed" in Microsoft-Windows-PrintService.
const printedEventID = 307

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	session := etw.NewRealTimeSession("SystemCheck-Print")
	prov, err := etw.ParseProvider("Microsoft-Windows-PrintService/Operational")
	if err != nil {
		// Fall back to the base provider name.
		prov, err = etw.ParseProvider("Microsoft-Windows-PrintService")
		if err != nil {
			return err
		}
	}
	if err := session.EnableProvider(prov); err != nil {
		return err
	}
	defer session.Stop()

	consumer := etw.NewRealTimeConsumer(ctx)
	consumer.FromSessions(session)
	if err := consumer.Start(); err != nil {
		return err
	}
	defer consumer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-consumer.Events:
			if !ok {
				return nil
			}
			if e.System.EventID != printedEventID {
				continue
			}
			d := e.EventData
			emit(wire.Event{
				Kind: "printjob",
				TS:   eventTime(e),
				Data: map[string]interface{}{
					"document": pick(d, "Param2", "DocumentName"),
					"user":     pick(d, "Param3", "User"),
					"printer":  pick(d, "Param5", "PrinterName"),
					"port":     pick(d, "Param6"),
					"bytes":    winutil.ToInt64(first(d, "Param7", "Size")),
					"pages":    winutil.ToInt64(first(d, "Param8", "Pages")),
				},
			})
		}
	}
}

func pick(d map[string]interface{}, keys ...string) string {
	return winutil.Str(first(d, keys...))
}

func first(d map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := d[k]; ok {
			return v
		}
	}
	return nil
}

func eventTime(e *etw.Event) time.Time {
	if t := e.System.TimeCreated.SystemTime; !t.IsZero() {
		return t.UTC()
	}
	return time.Now().UTC()
}
