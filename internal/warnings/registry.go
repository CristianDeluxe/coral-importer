package warnings

var all []*Warning

func Every(fn func(warning *Warning)) {
	for _, warning := range all {
		fn(warning)
	}
}

func register(warning *Warning) {
	all = append(all, warning)
}

func init() {
	register(UnsupportedDateFormat)
	register(UnsupportedUserProfileProvider)
	register(SSOIDMismatch)
}
