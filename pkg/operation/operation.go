package operation

// CfiOperation is the kind of operation we want to execute
type CfiOperation int

const (
	// CfiStatus represents a call to Status() (are we owner or standby)
	CfiStatus CfiOperation = iota

	// CfiPreempt represents a call to Preempt() (to take over IP address)
	CfiPreempt

	// CfiDestroy purges all routes we've created
	CfiDestroy
)
