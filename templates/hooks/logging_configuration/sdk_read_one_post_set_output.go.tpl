	// To avoid having logging configuration statusCode stuck in CREATING we can
	// update the status when calling sdkFind
	ko.Status.StatusCode = resp.LoggingConfiguration.Status.StatusCode