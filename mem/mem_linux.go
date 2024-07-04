package mem

import (
	"os"
)

func ClearBufferCache() error {
	f, err := os.Create("/proc/sys/vm/drop_caches")
	if err != nil {
		return err
	}
	f.WriteString("3")
	f.Close()
	return nil
}
