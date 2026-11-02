package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// notifyNewEpisodes tells the desktop that the library check found new
// episodes. A missing notifier is ignored: the text output already says it.
func notifyNewEpisodes(lines []string) {
	if len(lines) == 0 {
		return
	}
	if len(lines) > 5 {
		extra := len(lines) - 5
		lines = append(lines[:5], fmt.Sprintf("… und %d weitere", extra))
	}
	body := strings.Join(lines, "\n")
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("osascript", "-e", fmt.Sprintf(`display notification %q with title "orangeflix-dl"`, body))
	case "linux":
		path, err := exec.LookPath("notify-send")
		if err != nil {
			return
		}
		command = exec.Command(path, "orangeflix-dl", body)
	default:
		return
	}
	_ = command.Start()
}
