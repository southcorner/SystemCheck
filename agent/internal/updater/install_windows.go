//go:build windows

package updater

import (
	"os"
	"os/exec"
)

const serviceName = "SystemCheckAgent"

// Install replaces the running executable and schedules a service restart.
//
// On Windows a running .exe can be renamed but not overwritten, so we move the
// current binary aside, write the new one in its place, then launch a detached
// helper that restarts the service (which terminates and relaunches this
// process from the new binary).
func Install(data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.WriteFile(exe, data, 0o755); err != nil {
		// Best-effort rollback.
		_ = os.Rename(old, exe)
		return err
	}
	// Detached restart: wait briefly, then stop+start the service.
	cmd := exec.Command("cmd", "/C",
		"timeout /t 3 /nobreak >nul & sc stop "+serviceName+" & sc start "+serviceName)
	return cmd.Start()
}
