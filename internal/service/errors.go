package service

// ValidationError marks a client-supplied value as invalid for Connect error
// mapping. It must wrap only expected form or request validation failures; data
// store and dependency failures should remain ordinary errors.
type ValidationError struct {
	// Message is the client-safe validation failure text returned through Connect.
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}
