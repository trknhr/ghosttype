package model

func DrainAndLogEvents(ch <-chan ModelInitEvent, waitForHeavy bool) {
	if !waitForHeavy {
		go func() {
			for _ = range ch {
			}
		}()
		return
	}

	for {
		ev, ok := <-ch
		if !ok {
			return
		}
		if ev.Name == "llm" || ev.Name == "embedding" {
			if ev.Status == ModelReady || ev.Status == ModelError {
				break
			}
		}
	}
	go func() {
		for _ = range ch {
			// noop
		}
	}()
}
