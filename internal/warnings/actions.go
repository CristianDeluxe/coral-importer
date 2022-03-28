package warnings

var (
	// When an action is found in import data that does correspond to an action on
	// a comment, this warning is emitted.
	NonCommentAction = NewWarning()
)
