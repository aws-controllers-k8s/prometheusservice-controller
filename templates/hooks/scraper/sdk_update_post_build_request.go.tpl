
	// Same conversion as on create. This has to run post-build because the hook
	// point before it executes ahead of newUpdateRequestPayload, so input is nil there.
	if delta.DifferentAt("Spec.Configuration") && desired.ko.Spec.Configuration != nil {
		input.ScrapeConfiguration = &svcsdktypes.ScrapeConfigurationMemberConfigurationBlob{
			Value: []byte(*desired.ko.Spec.Configuration),
		}
	}
