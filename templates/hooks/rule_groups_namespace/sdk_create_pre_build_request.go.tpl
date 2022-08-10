	
    // Convert the string version of the configuration to a byte slice
	// because the API expects a base64 encoding. The conversion to base64
	// is handled automatically by k8s.
	if desired.ko.Spec.Configuration != nil {
		desired.ko.Spec.Data = []byte(*desired.ko.Spec.Configuration)
	}
