#ifndef SANTA_ABI_H
#define SANTA_ABI_H
#include <stdint.h>
#if defined(_WIN32)
#define SANTA_API __declspec(dllexport)
#define SANTA_CALL __cdecl
#else
#define SANTA_API __attribute__((visibility("default")))
#define SANTA_CALL
#endif
#ifdef __cplusplus
extern "C" {
#endif
#define SANTA_ABI_VERSION 2
/* v1 santa_draw remains available. v2 adds santa_draw_ex cancellation.
   cdecl, fixed-width integers, natural alignment, caller-owned buffers.
   Pairs are DIRECTED; adapters expand symmetric exclusions to both directions.
   No pointers retained. Calls are independent and thread-safe. Output untouched
   unless Success. All lengths are element counts, not bytes. Inputs/output must
   point to valid, non-overlapping memory. Cancellation is not exposed in v1. */
typedef struct SantaPair {
  uint32_t giver;
  uint32_t receiver;
} SantaPair;
typedef struct SantaBudget {
  uint64_t max_attempts;
  uint64_t max_work;
  uint64_t max_memory_bytes;
} SantaBudget;
typedef struct SantaCancellation {
  uint32_t* flag;
} SantaCancellation;
enum SantaStatus {
  SANTA_SUCCESS = 0,
  SANTA_INVALID_INPUT = 1,
  SANTA_UNSATISFIABLE = 2,
  SANTA_SEARCH_LIMIT_EXCEEDED = 3,
  SANTA_CANCELLED = 4,
  SANTA_INTERNAL_ERROR = 5,
  SANTA_RESOURCE_LIMIT_EXCEEDED = 6
};
SANTA_API uint32_t SANTA_CALL santa_abi_version(void);
SANTA_API int32_t SANTA_CALL santa_draw(uint32_t n, const SantaPair* pairs,
                                        uint64_t pair_count, SantaBudget budget,
                                        uint32_t* output, uint64_t output_count);
SANTA_API int32_t SANTA_CALL santa_draw_ex(uint32_t n, const SantaPair* pairs,
                                           uint64_t pair_count, SantaBudget budget,
                                           const SantaCancellation* cancellation,
                                           uint32_t* output, uint64_t output_count);
#ifdef __cplusplus
}
#endif
#endif
