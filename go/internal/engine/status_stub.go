//go:build !linux || !cgo

package engine

const (
	abiSuccess               Status = 0
	abiInvalidInput          Status = 1
	abiUnsatisfiable         Status = 2
	abiSearchLimitExceeded   Status = 3
	abiCancelled             Status = 4
	abiInternalError         Status = 5
	abiResourceLimitExceeded Status = 6
)
