package model

import (
	"log"
)

func DrainAndLogEvents(ch <-chan ModelInitEvent) {
	for ev := range ch {
		log.Printf("Drain event: %+v", ev)
	}
}
