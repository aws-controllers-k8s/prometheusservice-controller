
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

	// Scraper creation, update and deletion are asynchronous. A failed state is
	// terminal: AMP will not retry it, so requeueing forever would just hide the
	// reason from the user. This mirrors how the other resources in this
	// controller report status rather than using a generated synced condition,
	// which cannot express the failed states.
	if scraperHasFailed(&resource{ko}) {
		msg := "Scraper is in " + *ko.Status.Status.StatusCode + " status"
		if ko.Status.StatusReason != nil {
			msg += ": " + *ko.Status.StatusReason
		}
		rm.setStatusDefaults(ko)
		ackcondition.SetTerminal(&resource{ko}, corev1.ConditionTrue, &msg, nil)
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)
		return &resource{ko}, nil
	}

	if scraperInTransition(&resource{ko}) {
		// Setting the synced condition to false triggers a requeue, so there is
		// no need to return a requeue error here.
		rm.setStatusDefaults(ko)
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
		return &resource{ko}, nil
	}
