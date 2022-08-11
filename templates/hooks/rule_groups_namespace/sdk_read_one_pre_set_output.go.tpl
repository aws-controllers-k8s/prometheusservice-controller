
    // The data field stores the base64 encoding of the rule groups namespace.
    // However, to make the CR's more user friendly, we convert the base64 encoding to a 
    // string. We store it in a custom created field. 
    if resp.RuleGroupsNamespace.Data != nil {
        // Convert the base64 byte array to a human-readable string
        ruleGroupsNamespaceDataString := string(resp.RuleGroupsNamespace.Data)
        ko.Spec.Configuration = &ruleGroupsNamespaceDataString

        // Remove the data field as it is not user facing
        resp.RuleGroupsNamespace.Data = nil
    } else {
        ko.Spec.Configuration = nil
    }
