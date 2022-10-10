package lumberjack_test

import (
	"log"
	"time"

	"gopkg.in/huyinghuan/lumberjack.v4"
)

// To use lumberjack with the standard library's log package, just pass it into
// the SetOutput function when your application starts.
func Example() {
	l, _ := lumberjack.NewRoller(
		"/var/log/myapp/foo.log",
		&lumberjack.Options{
			MaxBackups: 3,
			MaxAge:     28 * time.Hour * 24, // 28 days
			Compress:   true,
			MaxSize:    500 * 1024 * 1024,
		})
	log.SetOutput(l)
}
