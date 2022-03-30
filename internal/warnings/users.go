package warnings

var (
	// When a user profile is found that is not supported, this warning is
	// emitted.
	UnsupportedUserProfileProvider = NewWarning("UnsupportedUserProfileProvider", "a user profile provider was encountered that was not supported, and was therefore skipped")
)
