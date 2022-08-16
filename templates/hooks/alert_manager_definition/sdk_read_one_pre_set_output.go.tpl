
    // Check the status of the alert manager definition
	if resp.AlertManagerDefinition.Status != nil {
		if resp.AlertManagerDefinition.Status.StatusCode != nil {
			ko.Status.StatusCode = resp.AlertManagerDefinition.Status.StatusCode
		} else {
			ko.Status.StatusCode = nil
		}
		if resp.AlertManagerDefinition.Status.StatusReason != nil {
			ko.Status.StatusReason = resp.AlertManagerDefinition.Status.StatusReason
		} else {
			ko.Status.StatusReason = nil
		}
	} else {
		ko.Status.StatusCode = nil
		ko.Status.StatusReason = nil

	}

    // When adding an invalid alert manager configuration, the AMP API has different behaviour
	// for different kinds of invalid input. For some invalid input, the API returns an error (e.g. ValidationException) 
	// instantly in the http response and we set the controller to terminal state. The specified
	// spec remains the same.
	// For other invalid inputs, the API first accepts the http request with a 200 code, and proceeds to
	// create/update the configuration but ultimately fails after around a minute because of an invalid config. So it
	// is possible for there to be a validation failure in an asynchronous update.
	// For these cases, the status will end up being "UPDATE_FAILED" or "CREATION_FAILED".
	// The behaviour of the API is as follows:
	//         - For a "CREATION_FAILED", the configuration will be empty.
	//         - For an "UPDATE_FAILED", the configuration will be the last valid one (or empty if CREATION_FAILED -> UPDATING -> UPDATE_FAILED).

	// However, from a K8s point of view, this can be confusing when the desired configuration is not the same
	// as the one shown in the resource after creating/updating. For example, resource says "UPDATE_FAILED" and
	// the spec has the previous ACTIVE configuration instead of the one that caused the failed update.

	// Hence, we should treat the asynchronous validation errors similarly to how the regular http validation
	// exceptions are treated in ACK. So when there is a failed creation/update, we don't change the configuration in the spec
	// that caused this failed status, and also set to terminal error.

	// When a failed status occurs, we skip setting the configuration field to be what the API returns,
	// and instead keep it to be what the user desires. We only do this once right after a resource becomes
	// failed and not after because otherwise it would prevent update calls since the configuration wouldn't ever change from failed.
	// So we only want to prevent the configuration changing when the status changed from creating/updating to failed. After, when the user
	// updates the configuration, then it should update.

	// This is done by by checking if the returned status is failed while the current resource isn't.
	if (alertManagerDefinitionStatusFailed(&resource{ko}) && !alertManagerDefinitionStatusFailed(r) &&
		alertManagerDefinitionValidationError(&resource{ko})) {
		msg := "Alert Manager Definition is in '" + *ko.Status.StatusCode + "' status because of a validating error"
		rm.setStatusDefaults(ko)

		ackcondition.SetTerminal(&resource{ko}, corev1.ConditionTrue, &msg, nil)
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)

		return &resource{ko}, nil

	}

	// The data field stores the base64 encoding of the alert manager definition.
	// However, to make the CR's more user friendly, we convert the base64 encoding to a
	// string. We store it in a custom created field.
	if resp.AlertManagerDefinition.Data != nil {
		// Convert the base64 byte array to a human-readable string
		alertManagerDefinitionDataString := string(resp.AlertManagerDefinition.Data)

		ko.Spec.Configuration = &alertManagerDefinitionDataString
		if err != nil {
			return nil, err
		}
	} else {
		ko.Spec.Configuration = nil
	}

	// if there is a read call and the status has already failed before, then the if
	// statements above setting the config field would trigger an update call because the server response
	// will not match with the desired configuration. This is expected and needed for when a user updates the
	// resource once a status has become failed.

	// There is one edge case however where this isn't true. When a user creates a valid config but then an invalid update,
	// then the server-side resource will be "UPDATE_FAILED" but the server-side configuration will be the first valid configuration. In the scenario,
	// if a user changes updates the config in their spec back to the original valid one, then the desired and server response will be the same. With no difference,
	// no update call will be triggerred, and the resource will remain in UPDATE_FAILED. As a work around for this edge case, we set the config to nil to
	// force an update call.
    if alertManagerDefinitionStatusFailed(r){
        ko.Spec.Configuration = nil
    }

	if alertManagerDefinitionUpdating(&resource{ko}) {
		// Setting resource synced condition to false will trigger a requeue of
		// the resource. No need to return a requeue error here.
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
		return &resource{ko}, nil
	}
