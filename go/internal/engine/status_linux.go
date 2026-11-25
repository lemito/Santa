//go:build linux && cgo

package engine

/*
#cgo linux pkg-config: santa_native
#include "santa/abi.h"
*/
import "C"

const (
	abiSuccess               Status = C.SANTA_SUCCESS
	abiInvalidInput          Status = C.SANTA_INVALID_INPUT
	abiUnsatisfiable         Status = C.SANTA_UNSATISFIABLE
	abiSearchLimitExceeded   Status = C.SANTA_SEARCH_LIMIT_EXCEEDED
	abiCancelled             Status = C.SANTA_CANCELLED
	abiInternalError         Status = C.SANTA_INTERNAL_ERROR
	abiResourceLimitExceeded Status = C.SANTA_RESOURCE_LIMIT_EXCEEDED
)
