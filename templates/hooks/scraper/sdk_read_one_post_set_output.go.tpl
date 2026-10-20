
	// The API nests the state inside a Status object, which is flattened out of the
	// model (see ignore.field_paths), so copy it onto Status.StatusCode here. This
	// runs post-set-output, after the generated code has applied the response, so it
	// reflects what AWS just returned rather than the previous reconcile's value.
	if resp.Scraper.Status != nil && resp.Scraper.Status.StatusCode != "" {
		ko.Status.StatusCode = aws.String(string(resp.Scraper.Status.StatusCode))
	} else {
		ko.Status.StatusCode = nil
	}

	// A failed state is terminal: AWS will not retry it, and synced.when only ever
	// reports ACTIVE as synced, so without this the resource would requeue forever
	// with the reason buried in its status.
	if scraperHasFailed(&resource{ko}) {
		msg := "Scraper is in " + *ko.Status.StatusCode + " status"
		if ko.Status.StatusReason != nil {
			msg += ": " + *ko.Status.StatusReason
		}
		ackcondition.SetTerminal(&resource{ko}, corev1.ConditionTrue, &msg, nil)
	}
