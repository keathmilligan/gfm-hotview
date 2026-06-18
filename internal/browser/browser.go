// Package browser opens a URL in the user's default browser using the
// OS-native mechanism. Failures are non-fatal to the caller.
package browser

import (
	"os/exec"
	"runtime"
)

// Open attempts to open url in the default browser. It returns an error if the
// launch command could not be started; callers typically just log and continue.
func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, *bsd
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
