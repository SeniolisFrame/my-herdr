package herdr

import (
	"fmt"
	"os/exec"
)

func FocusFrom(pane, dir string) error {
	if pane == "" {
		return fmt.Errorf("not running in a herdr pane")
	}
	return exec.Command(Bin(), "pane", "focus", "--pane", pane, "--direction", dir).Run()
}
