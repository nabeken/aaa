package agent

import "log"

func Debug(v ...interface{}) {
	log.Print(append([]interface{}{"DEBUG: "}, v...)...)
}
