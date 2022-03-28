package warnings

var (
	// When a user profile is found that is not supported, this warning is
	// emitted.
	UnsupportedUserProfileProvider = NewWarning()
)
