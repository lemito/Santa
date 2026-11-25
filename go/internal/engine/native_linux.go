//go:build linux && cgo

package engine

/*
#cgo linux pkg-config: santa_native
#include <stdlib.h>
#include "santa/abi.h"
static inline void santa_set_cancel(uint32_t* flag) {
  __atomic_store_n(flag, 1, __ATOMIC_RELAXED);
}
*/
import "C"
import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/google/uuid"
)

func nativeDraw(ctx context.Context, ids []uuid.UUID, exclusions []Exclusion, budget Budget) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{Status: Cancelled}, err
	}
	index := make(map[uuid.UUID]uint32, len(ids))
	for i, id := range ids {
		index[id] = uint32(i)
	}
	pairs := make([]C.SantaPair, 0, len(exclusions)*2)
	for _, p := range exclusions {
		a, okA := index[p.First]
		b, okB := index[p.Second]
		if !okA || !okB {
			return Result{Status: InvalidInput}, fmt.Errorf("unknown exclusion")
		}
		pairs = append(pairs, C.SantaPair{giver: C.uint32_t(a), receiver: C.uint32_t(b)}, C.SantaPair{giver: C.uint32_t(b), receiver: C.uint32_t(a)})
	}
	out := make([]C.uint32_t, len(ids))
	flag := (*C.uint32_t)(C.calloc(1, C.size_t(4)))
	if flag == nil {
		return Result{Status: ResourceLimitExceeded}, fmt.Errorf("allocate cancellation flag")
	}
	defer C.free(unsafe.Pointer(flag))
	done := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		select {
		case <-ctx.Done():
			C.santa_set_cancel(flag)
		case <-done:
		}
	}()
	cancellation := C.SantaCancellation{flag: flag}
	rawStatus := C.santa_draw_ex(C.uint32_t(len(ids)), ptrPair(pairs), C.uint64_t(len(pairs)), C.SantaBudget{max_attempts: C.uint64_t(budget.MaxAttempts), max_work: C.uint64_t(budget.MaxWork), max_memory_bytes: C.uint64_t(budget.MaxMemoryBytes)}, &cancellation, ptrUint(out), C.uint64_t(len(out)))
	close(done)
	watcher.Wait()
	status := statusFromABI(rawStatus)
	result := Result{Status: status}
	if status != Success {
		return result, nil
	}
	result.Assignments = make(map[uuid.UUID]uuid.UUID, len(ids))
	for i, receiver := range out {
		if uint64(receiver) >= uint64(len(ids)) {
			return Result{Status: InternalError}, ErrInvalidResult
		}
		result.Assignments[ids[i]] = ids[receiver]
	}
	if err := ValidateAssignments(Input{Participants: ids, Exclusions: exclusions}, result.Assignments); err != nil {
		return Result{Status: InternalError}, err
	}
	return result, nil
}

func ptrPair(v []C.SantaPair) *C.SantaPair {
	if len(v) == 0 {
		return nil
	}
	return &v[0]
}

func statusFromABI(status C.int32_t) Status {
	switch status {
	case C.SANTA_SUCCESS:
		return Success
	case C.SANTA_INVALID_INPUT:
		return InvalidInput
	case C.SANTA_UNSATISFIABLE:
		return Unsatisfiable
	case C.SANTA_SEARCH_LIMIT_EXCEEDED:
		return SearchLimitExceeded
	case C.SANTA_CANCELLED:
		return Cancelled
	case C.SANTA_INTERNAL_ERROR:
		return InternalError
	case C.SANTA_RESOURCE_LIMIT_EXCEEDED:
		return ResourceLimitExceeded
	default:
		return InternalError
	}
}

func ptrUint(v []C.uint32_t) *C.uint32_t {
	if len(v) == 0 {
		return nil
	}
	return &v[0]
}

func init() {
	if C.santa_abi_version() != C.SANTA_ABI_VERSION {
		panic("unsupported santa ABI")
	}
}
