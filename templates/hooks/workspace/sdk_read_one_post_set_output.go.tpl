    if workspaceCreating(&resource{ko}) {
		return &resource{ko}, requeueWaitWhileCreating
	}
