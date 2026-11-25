#include <santa/abi.h>
#include <santa/engine.h>

#include <atomic>
#include <chrono>
#include <cstdlib>
#include <new>

#include <yasl/core/utility.h>
#include <yasl/io/file.h>

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
class DrawLog final {
 public:
  DrawLog(uint32 n, uint64 pair_count, SantaBudget budget)
      : enabled_(logging_enabled()),
        n_(n),
        pair_count_(pair_count),
        budget_(budget),
        started_(std::chrono::steady_clock::now()) {
    emit_started();
  }

  ~DrawLog() noexcept { emit_finished(); }

  void finish(int32 status, uint64 attempts = 0) {
    status_ = status;
    attempts_ = attempts;
  }

 private:
  static bool logging_enabled() {
    const char* value = std::getenv("SANTA_NATIVE_LOG");
    return value != nullptr && value[0] == '1';
  }

  uint64 elapsed_us() const {
    return static_cast<uint64>(std::chrono::duration_cast<std::chrono::microseconds>(
                                    std::chrono::steady_clock::now() - started_)
                                    .count());
  }

  void emit_started() noexcept {
    if (!enabled_) return;
    try {
      yasl::io::err().println(
          "santa_native_draw_started n={} exclusions={} max_attempts={} max_work={} max_memory_bytes={}",
          n_, pair_count_, budget_.max_attempts, budget_.max_work,
          budget_.max_memory_bytes);
    } catch (...) {
    }
  }

  void emit_finished() noexcept {
    if (!enabled_) return;
    try {
      yasl::io::err().println(
          "santa_native_draw_finished n={} exclusions={} status={} attempts={} duration_us={}",
          n_, pair_count_, status_, attempts_, elapsed_us());
    } catch (...) {
    }
  }

  bool enabled_;
  uint32 n_;
  uint64 pair_count_;
  SantaBudget budget_;
  std::chrono::steady_clock::time_point started_;
  int32 status_ = SANTA_INTERNAL_ERROR;
  uint64 attempts_ = 0;
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
  DrawLog draw_log(n, pair_count, budget);
  if (!output || output_count < n || (pair_count && !pairs) ||
      (cancellation && !cancellation->flag) ||
      pair_count > yasl::max_value<size_t>() / sizeof(SantaPair))
  {
    draw_log.finish(SANTA_INVALID_INPUT);
    return SANTA_INVALID_INPUT;
  }
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
    draw_log.finish(status, result.attempts);
    if (result.status == santa::Status::kSuccess)
      for (uint32 i = 0; i < n; ++i) output[i] = result.receivers[i];
    return status;
  } catch (const std::bad_alloc&) {
    draw_log.finish(SANTA_RESOURCE_LIMIT_EXCEEDED);
    return SANTA_RESOURCE_LIMIT_EXCEEDED;
  } catch (const yasl::BadAlloc&) {
    draw_log.finish(SANTA_RESOURCE_LIMIT_EXCEEDED);
    return SANTA_RESOURCE_LIMIT_EXCEEDED;
  } catch (...) {
    draw_log.finish(SANTA_INTERNAL_ERROR);
    return SANTA_INTERNAL_ERROR;
  }
}
