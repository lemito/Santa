#include <santa/abi.h>
#include <santa/engine.h>

#include <new>

#include <yasl/core/utility.h>

static_assert(sizeof(SantaPair) == 8);
static_assert(sizeof(SantaBudget) == 24);
int32 to_abi_status(const santa::Status status) {
  switch (status) {
    case santa::Status::kSuccess:
      return SANTA_SUCCESS;
    case santa::Status::kInvalidInput:
      return SANTA_INVALID_INPUT;
    case santa::Status::kUnsatisfiable:
      return SANTA_UNSATISFIABLE;
    case santa::Status::kSearchLimitExceeded:
      return SANTA_SEARCH_LIMIT_EXCEEDED;
    case santa::Status::kCancelled:
      return SANTA_CANCELLED;
    case santa::Status::kResourceLimitExceeded:
      return SANTA_RESOURCE_LIMIT_EXCEEDED;
    case santa::Status::kInternalError:
      return SANTA_INTERNAL_ERROR;
  }
  return SANTA_INTERNAL_ERROR;
}
class FlagCancel final : public santa::ICancel {
 public:
  explicit FlagCancel(uint32* flag) : flag_(flag) {}
  bool cancelled() const override {
    return flag_ != nullptr &&
           std::atomic_ref<uint32>(*flag_).load(std::memory_order_relaxed) != 0;
  }

 private:
  uint32* flag_;
};
uint32 SANTA_CALL santa_abi_version(void) { return SANTA_ABI_VERSION; }
int32 SANTA_CALL santa_draw(uint32 n, const SantaPair* pairs,
                              uint64 pair_count, SantaBudget budget,
                              uint32* output, uint64 output_count) {
  return santa_draw_ex(n, pairs, pair_count, budget, nullptr, output, output_count);
}
int32 SANTA_CALL santa_draw_ex(uint32 n, const SantaPair* pairs,
                               uint64 pair_count, SantaBudget budget,
                               const SantaCancellation* cancellation,
                               uint32* output, uint64 output_count) {
  if (!output || output_count < n || (pair_count && !pairs) ||
      (cancellation && !cancellation->flag) ||
      pair_count > yasl::max_value<size_t>() / sizeof(SantaPair))
    return SANTA_INVALID_INPUT;
  try {
    santa::OsRandom random;
    yasl::Vector<santa::Pair> native_pairs;
    native_pairs.reserve(static_cast<size_t>(pair_count));
    for (uint64 i = 0; i < pair_count; ++i)
      native_pairs.push_back({pairs[i].giver, pairs[i].receiver});
    const santa::Budget native_budget{
        budget.max_attempts, budget.max_work, budget.max_memory_bytes};
    FlagCancel cancel(cancellation ? cancellation->flag : nullptr);
    const auto result = santa::draw(
        n, native_pairs.span(), native_budget, random,
        cancellation ? static_cast<santa::ICancel*>(&cancel) : nullptr);
    const auto status = to_abi_status(result.status);
    if (result.status == santa::Status::kSuccess)
      for (uint32 i = 0; i < n; ++i) output[i] = result.receivers[i];
    return status;
  } catch (const std::bad_alloc&) {
    return SANTA_RESOURCE_LIMIT_EXCEEDED;
  } catch (const yasl::BadAlloc&) {
    return SANTA_RESOURCE_LIMIT_EXCEEDED;
  } catch (...) {
    return SANTA_INTERNAL_ERROR;
  }
}
