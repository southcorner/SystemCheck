//go:build windows

package dns

import (
	"context"
	"time"

	"github.com/0xrawsec/golang-etw/etw"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/winutil"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// dnsQueryEventID is "DNS query issued" from Microsoft-Windows-DNS-Client.
const dnsQueryEventID = 3006

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	session := etw.NewRealTimeSession("SystemCheck-DNS")
	prov, err := etw.ParseProvider("Microsoft-Windows-DNS-Client")
	if err != nil {
		return err
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
			if e.System.EventID != dnsQueryEventID {
				continue
			}
			domain := winutil.Str(e.EventData["QueryName"])
			if domain == "" {
				continue
			}
			pid := e.System.Execution.ProcessID
			proc := winutil.ProcessName(pid)
			if c.excluded(domain, proc) {
				continue
			}
			emit(wire.Event{
				Kind: "dns",
				TS:   eventTime(e),
				Data: map[string]interface{}{
					"domain":  domain,
					"process": proc,
					"pid":     pid,
				},
			})
		}
	}
}

func eventTime(e *etw.Event) time.Time {
	if t := e.System.TimeCreated.SystemTime; !t.IsZero() {
		return t.UTC()
	}
	return time.Now().UTC()
}
