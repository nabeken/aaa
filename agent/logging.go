package agent

import "log"

func Debug(v ...any) {
	log.Print(append([]any{"DEBUG: "}, v...)...)
}
