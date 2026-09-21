
	// The API takes the scrape configuration as a base64-encoded blob wrapped in a
	// union, but users supply plain YAML in Spec.Configuration, matching the rule
	// groups and alert manager resources. The base64 conversion is handled by k8s.
	if desired.ko.Spec.Configuration != nil {
		input.ScrapeConfiguration = &svcsdktypes.ScrapeConfigurationMemberConfigurationBlob{
			Value: []byte(*desired.ko.Spec.Configuration),
		}
	}
