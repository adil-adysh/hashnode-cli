package diff

// Diffing utilities between local and remote state.

// ChangeType represents the type of change detected.
type ChangeType int

const (
	Create ChangeType = iota
	Update
	Delete
	Skip
)

// Change is a single planned change
type Change struct {
	Type ChangeType
	Path string
}

// Compute is a stub for computing diffs
func Compute(local interface{}, remote interface{}) ([]Change, error) {
	return nil, nil
}
