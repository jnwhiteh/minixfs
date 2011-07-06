package main

import (
	"log"
)

func debug(fmt string) {
        if *showDebug {
                log.Print(fmt)
        }
}

func debugf(fmt string, args ... interface{}) {
        if *showDebug {
                log.Printf(fmt, args...)
        }
}
