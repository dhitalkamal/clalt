package account

import "fmt"

// NotFoundError means no account has the id.
type NotFoundError struct{ ID string }

// Error gives the message the store has always used for a missing account.
func (e *NotFoundError) Error() string { return fmt.Sprintf("account %q not found", e.ID) }

// LabelTakenError means another account already has the label. ID is that
// account. Hidden says it was removed: its label is still held, and logging in
// with the label brings it back.
type LabelTakenError struct {
	Label  string
	ID     string
	Hidden bool
}

// Error says who holds the label, and that they were removed if so.
func (e *LabelTakenError) Error() string {
	if e.Hidden {
		return fmt.Sprintf("an account labeled %q already exists (it was removed, and its label is still taken)", e.Label)
	}
	return fmt.Sprintf("an account labeled %q already exists", e.Label)
}

func errNoAccount(id string) error { return &NotFoundError{ID: id} }
