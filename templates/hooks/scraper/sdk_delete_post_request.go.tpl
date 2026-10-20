
	// DeleteScraper only starts the deletion. Requeue so the finalizer is dropped
	// only once DescribeScraper stops finding the scraper, rather than leaving the
	// CR gone while AWS is still tearing the scraper down.
	if err == nil {
		return nil, requeueWaitWhileDeleting
	}
