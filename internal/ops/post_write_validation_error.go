package ops

import "fmt"

// PostWriteValidationError indicates a mutation persisted but state validation
// failed immediately afterward.
type PostWriteValidationError struct {
	Err error
}

func (e *PostWriteValidationError) Error() string {
	return fmt.Sprintf("state validation failed after write: %v", e.Err)
}

func (e *PostWriteValidationError) Unwrap() error {
	return e.Err
}
