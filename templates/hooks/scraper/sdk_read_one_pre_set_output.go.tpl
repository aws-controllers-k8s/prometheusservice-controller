
	// Mirror of the create hook: surface the returned configuration blob as the
	// human-readable string the user supplied, so no drift is reported against it.
	if resp.Scraper.ScrapeConfiguration != nil {
		blob, ok := resp.Scraper.ScrapeConfiguration.(*svcsdktypes.ScrapeConfigurationMemberConfigurationBlob)
		if !ok {
			return nil, ackerr.NewTerminalError(fmt.Errorf(
				"unsupported scrape configuration type %T returned by DescribeScraper",
				resp.Scraper.ScrapeConfiguration,
			))
		}
		configuration := string(blob.Value)
		ko.Spec.Configuration = &configuration
	} else {
		ko.Spec.Configuration = nil
	}
