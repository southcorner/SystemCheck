//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const serviceName = "SystemCheckAgent"

// Windows process creation flags.
const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// Install stages the new binary and hands the swap+restart to a fully detached
// helper process, so it survives the service being stopped.
//
// A running .exe can't be overwritten while the service holds it, and a child
// process spawned by the service is torn down when the service stops - which is
// why an in-process swap + child restart failed to apply. Instead we write the
// new binary alongside as agent.exe.new, then launch a DETACHED cmd (its own
// process group, no parent ties) that: waits, stops the service (releasing the
// exe lock), moves the new binary into place, and starts the service.
func Install(data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	newPath := exe + ".new"
	if err := os.WriteFile(newPath, data, 0o755); err != nil {
		return err
	}
	// ping is used as a portable ~3s sleep (timeout needs a console).
	script := fmt.Sprintf(
		`ping 127.0.0.1 -n 4 >nul & sc stop %s >nul 2>&1 & `+
			`move /y "%s" "%s" >nul & sc start %s >nul 2>&1`,
		serviceName, newPath, exe, serviceName)
	cmd := exec.Command("cmd", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
	return cmd.Start()
}
