#pragma once

#include <yasl/cont/seq/vector.h>
#include <yasl/core/types.h>

namespace santa {
struct Pair {
  uint32 giver;
  uint32 receiver;
};
struct Budget {
  uint64 max_attempts;
  uint64 max_work;
  uint64 max_memory_bytes;
};
enum class Status : int32 {
  kSuccess = 0,
  kInvalidInput = 1,
  kUnsatisfiable = 2,
  kSearchLimitExceeded = 3,
  kCancelled = 4,
  kInternalError = 5,
  kResourceLimitExceeded = 6
};
class IRandom {
 public:
  virtual ~IRandom() = default;
  virtual uint64 next() = 0;
};
class ICancel {
 public:
  virtual ~ICancel() = default;
  virtual bool cancelled() const = 0;
};
class OsRandom final : public IRandom {
 public:
  uint64 next() override;
};
struct Result {
  Status status;
  yasl::Vector<uint32> receivers;
  // Hall witness: these givers have fewer available receivers than their count.
  yasl::Vector<uint32> affected_givers;
  uint64 attempts = 0;
};
// All indices are snapshot-local. Duplicate directed pairs are harmless.
Result draw(uint32 n, yasl::Span<const Pair> exclusions, Budget budget,
            IRandom& random, ICancel* cancel = nullptr);
bool valid(uint32 n, yasl::Span<const Pair> exclusions,
           yasl::Span<const uint32> receivers);
yasl::Vector<Pair> symmetric(yasl::Span<const Pair> pairs);
}  // namespace santa
