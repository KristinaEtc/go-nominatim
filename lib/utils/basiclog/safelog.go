package basiclog

import (
	"fmt"
)

// TODO: create directory in /var/log, if in linux:
// if runtime.GOOS == "linux" {
//os.Mkdir("."+string(filepath.Separator)+LogDir, 0766)
//eventlog & output debug string for windows
//syslog & stderr for linux

func SafeLog(msg string) {
	fmt.Println(msg)
}
