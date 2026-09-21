
	// A scraper already in DELETING would reject another DeleteScraper, so requeue
	// without calling it again.
	if scraperDeleting(r) {
		return nil, requeueWaitWhileDeleting
	}
