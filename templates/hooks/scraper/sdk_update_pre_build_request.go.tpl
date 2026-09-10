
	// UpdateScraper accepts no source, so a source change can never converge.
	// Fail terminally rather than reconciling forever; the user must replace the
	// scraper. This is deliberately not is_immutable, whose CEL rule would reject
	// the adoption reconciler's own whole-spec write-back.
	if delta.DifferentAt("Spec.Source") {
		return nil, ackerr.NewTerminalError(fmt.Errorf(
			"Spec.Source is immutable; AMP requires replacing the scraper to change its source",
		))
	}
