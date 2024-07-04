package mem

import (
	"os/exec"
)

func ClearBufferCache() error {
	return exec.Command("/usr/sbin/purge").Run()
}
