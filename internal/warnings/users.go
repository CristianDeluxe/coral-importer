package warnings

var (
	// When a user profile is found that is not supported, this warning is
	// emitted.
	UnsupportedUserProfileProvider = NewWarning("UnsupportedUserProfileProvider", "a user profile provider was encountered that was not supported, and was therefore skipped")

	// Whe a user profile is found that does not have the same ID as it's id, this
	// warning is emitted.
	SSOIDMismatch = NewWarning("SSOIDMismatch", "a user profile was found that had a different ID from it's SSO ID")
)
