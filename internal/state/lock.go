package state

// Lock file parser/stubs for .hnsync.lock

// Entry represents a single lock file line
type Entry struct {
	Type       string
	Hash       string
	Identifier string
	RemoteID   string
	Status     string
}

// ParseLock is a stub for parsing the lock file.
func ParseLock(path string) ([]Entry, error) {
	// TODO: implement parser
	return nil, nil
}
