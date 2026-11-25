package engine

type Status int

const (
	Success               Status = abiSuccess
	InvalidInput          Status = abiInvalidInput
	Unsatisfiable         Status = abiUnsatisfiable
	SearchLimitExceeded   Status = abiSearchLimitExceeded
	Cancelled             Status = abiCancelled
	InternalError         Status = abiInternalError
	ResourceLimitExceeded Status = abiResourceLimitExceeded
)

func (s Status) String() string {
	switch s {
	case Success:
		return "success"
	case InvalidInput:
		return "invalid_input"
	case Unsatisfiable:
		return "unsatisfiable"
	case SearchLimitExceeded:
		return "search_limit"
	case Cancelled:
		return "cancelled"
	case InternalError:
		return "internal_error"
	case ResourceLimitExceeded:
		return "resource_limit"
	default:
		return "unknown"
	}
}
