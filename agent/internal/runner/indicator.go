package runner

import "log"

// showIndicator surfaces the transparency indicator to the user. Monitoring is
// never covert: staff must be able to tell when it is active.
//
// MVP prints a clear status line to the agent log / console. The production
// indicator is a persistent system-tray icon with an "active/paused" state and a
// link to the monitoring policy; that lands with the tray work in Phase 1 polish.
func showIndicator(active bool) {
	if active {
		log.Printf("INDICATOR: SystemCheck monitoring is ACTIVE on this device (see your monitoring policy).")
	} else {
		log.Printf("INDICATOR: SystemCheck is installed but monitoring is NOT active (awaiting consent).")
	}
}
