//go:build windows

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const serviceName = "SystemCheckAgent"

// Windows process creation flags.
const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// swapScript stops the service, waits for it to REALLY stop, swaps the binary,
// and starts it again - logging each step so a failed rollout is diagnosable.
//
// The critical detail is the wait loop: "sc stop" only *requests* a stop and
// returns immediately. Without waiting for the service to reach STOPPED, the
// move below runs while the .exe is still locked, fails, and the service comes
// back up on the OLD binary - which then immediately re-downloads the update,
// restarts, and repeats. That is a fleet-wide download/restart storm, so the
// wait and the move retries are both load-bearing.
const swapScript = `@echo off
setlocal
set "SVC=__SVC__"
set "NEW=__NEW__"
set "EXE=__EXE__"
set "LOG=__LOG__"
echo [%DATE% %TIME%] update: stopping %SVC% >>"%LOG%"
ping 127.0.0.1 -n 3 >nul
sc stop %SVC% >nul 2>&1
rem sc stop is asynchronous: wait (up to ~60s) for the service to actually stop
rem so the exe lock is released before we replace it.
for /l %%i in (1,1,60) do (
  sc query %SVC% | findstr /c:"STOPPED" >nul 2>&1 && goto :swap
  ping 127.0.0.1 -n 2 >nul
)
echo [%DATE% %TIME%] update: WARNING service never reported STOPPED, trying swap anyway >>"%LOG%"
:swap
rem Retry the move: the image can stay briefly locked after the service stops.
for /l %%i in (1,1,30) do (
  move /y "%NEW%" "%EXE%" >nul 2>&1 && goto :launch
  ping 127.0.0.1 -n 2 >nul
)
echo [%DATE% %TIME%] update: ERROR could not replace %EXE% (still locked); starting old binary >>"%LOG%"
:launch
echo [%DATE% %TIME%] update: starting %SVC% >>"%LOG%"
sc start %SVC% >nul 2>&1
echo [%DATE% %TIME%] update: done >>"%LOG%"
`

// Install stages the new binary and hands the swap+restart to a fully detached
// helper process, so it survives the service being stopped.
//
// A running .exe can't be overwritten while the service holds it, and a child
// process spawned by the service is torn down when the service stops - which is
// why an in-process swap + child restart failed to apply. Instead we write the
// new binary alongside as agent.exe.new plus a swap script, then launch a
// DETACHED cmd (its own process group, no parent ties) to run it.
func Install(data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	newPath := exe + ".new"
	if err := os.WriteFile(newPath, data, 0o755); err != nil {
		return err
	}

	script := swapScript
	for from, to := range map[string]string{
		"__SVC__": serviceName,
		"__NEW__": newPath,
		"__EXE__": exe,
		"__LOG__": filepath.Join(dir, "agent-update.log"),
	} {
		script = strings.ReplaceAll(script, from, to)
	}
	// cmd is happiest with CRLF, especially around labels/goto.
	script = strings.ReplaceAll(script, "\n", "\r\n")

	// Fixed name so updates overwrite it instead of accumulating files.
	scriptPath := filepath.Join(dir, "agent-update.cmd")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return err
	}

	cmd := exec.Command("cmd", "/C", scriptPath)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
	return cmd.Start()
}
