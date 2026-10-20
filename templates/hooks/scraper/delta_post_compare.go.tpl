	// Spec.Source.EKSConfiguration.SecurityGroupIDs is excluded from the generated
	// comparison above (compare.is_ignored in generator.yaml). AMP assigns a security
	// group when the field is omitted, so comparing an empty desired value against the
	// security group DescribeScraper reports would register a difference on every
	// reconcile, which the Spec.Source guard in sdkUpdate rejects as an attempt to
	// change an immutable source.
	//
	// Compare it here instead, and only when the user actually specified security
	// groups, so a real attempt to change them is still reported rather than silently
	// ignored.
	if a.ko.Spec.Source != nil && a.ko.Spec.Source.EKSConfiguration != nil &&
		len(a.ko.Spec.Source.EKSConfiguration.SecurityGroupIDs) > 0 {
		var latestSecurityGroupIDs []*string
		if b.ko.Spec.Source != nil && b.ko.Spec.Source.EKSConfiguration != nil {
			latestSecurityGroupIDs = b.ko.Spec.Source.EKSConfiguration.SecurityGroupIDs
		}
		if !ackcompare.SliceStringPEqual(
			a.ko.Spec.Source.EKSConfiguration.SecurityGroupIDs,
			latestSecurityGroupIDs,
		) {
			delta.Add(
				"Spec.Source.EKSConfiguration.SecurityGroupIDs",
				a.ko.Spec.Source.EKSConfiguration.SecurityGroupIDs,
				latestSecurityGroupIDs,
			)
		}
	}
