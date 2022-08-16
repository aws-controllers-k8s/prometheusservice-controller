	// Can't delete alert manager definition in non-(ACTIVE/CREATION_FAILED/UPDATE_FAILED) state
    // Otherwise, API will return a 409 and ConflictException
    if !alertManagerDefinitionStatusFailed(r) && !alertManagerDefinitionActive(r){
		msg := "Cannot delete alert manager definition as the status is not ACTIVE/CREATION_FAILED/UPDATE_FAILED, current status=" + string(*r.ko.Status.StatusCode)
		ackcondition.SetSynced(r, corev1.ConditionFalse, &msg, nil)
		return r, ackrequeue.NeededAfter(
			errors.New(msg),
			ackrequeue.DefaultRequeueAfterDuration,
		)
	}