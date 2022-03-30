package warnings

var (
	// When an unsupported date format is encountered, this warning is emitted.
	UnsupportedDateFormat = NewWarning("UnsupportedDateFormat", "a date format was encountered and was not processed")
)
