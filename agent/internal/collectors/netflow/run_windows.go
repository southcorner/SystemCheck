//go:build windows

package netflow

import (
	"context"
	"sync"
	"time"

	"github.com/0xrawsec/golang-etw/etw"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/winutil"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Kernel-Network event IDs. Send/recv for TCP and UDP over IPv4/IPv6.
var (
	sendIDs = map[uint16]bool{10: true, 26: true, 42: true, 58: true}
	recvIDs = map[uint16]bool{11: true, 27: true, 43: true, 59: true}
)

type counter struct {
	sent int64
	recv int64
}

func (c *Collector) run(ctx context.Context, emit collectors.Emit) error {
	session := etw.NewRealTimeSession("SystemCheck-Netflow")
	prov, err := etw.ParseProvider("Microsoft-Windows-Kernel-Network")
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

	var mu sync.Mutex
	counters := map[uint32]*counter{}

	rollup := time.Duration(c.rollupSec()) * time.Second
	ticker := time.NewTicker(rollup)
	defer ticker.Stop()

	flush := func() {
		mu.Lock()
		snapshot := counters
		counters = map[uint32]*counter{}
		mu.Unlock()
		for pid, ct := range snapshot {
			if ct.sent == 0 && ct.recv == 0 {
				continue
			}
			emit(wire.Event{
				Kind: "netflow",
				TS:   time.Now().UTC(),
				Data: map[string]interface{}{
					"process":    winutil.ProcessName(pid),
					"pid":        pid,
					"sent_bytes": ct.sent,
					"recv_bytes": ct.recv,
					"window_sec": c.rollupSec(),
				},
			})
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return nil
		case <-ticker.C:
			flush()
		case e, ok := <-consumer.Events:
			if !ok {
				flush()
				return nil
			}
			id := e.System.EventID
			isSend := sendIDs[id]
			isRecv := recvIDs[id]
			if !isSend && !isRecv {
				continue
			}
			size := winutil.ToInt64(e.EventData["size"])
			if size == 0 {
				continue
			}
			pid := uint32(winutil.ToInt64(e.EventData["PID"]))
			if pid == 0 {
				pid = e.System.Execution.ProcessID
			}
			mu.Lock()
			ct := counters[pid]
			if ct == nil {
				ct = &counter{}
				counters[pid] = ct
			}
			if isSend {
				ct.sent += size
			} else {
				ct.recv += size
			}
			mu.Unlock()
		}
	}
}
