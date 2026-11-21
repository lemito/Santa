#include <santa/engine.h>
#include <sys/random.h>

#include <cerrno>

#include <yasl/core/bits.h>
#include <yasl/core/exception.h>
#include <yasl/core/utility.h>

namespace santa {
namespace {
struct Limit {};
class Work {
 public:
  explicit Work(uint64 count) : left_(count) {}
  void use(uint64 amount = 1) {
    if (amount > left_) throw Limit{};
    left_ -= amount;
  }

 private:
  uint64 left_;
};
uint64 bounded(IRandom& random, uint64 bound, Work& work) {
  const auto threshold = (uint64{0} - bound) % bound;
  for (;;) {
    work.use();
    const auto value = random.next();
    if (value >= threshold) return value % bound;
  }
}
}  // namespace
uint64 OsRandom::next() {
  uint64 value;
  auto* bytes = reinterpret_cast<unsigned char*>(&value);
  size_t done = 0;
  while (done < sizeof(value)) {
    const auto read = getrandom(bytes + done, sizeof(value) - done, 0);
    if (read < 0 && errno == EINTR) continue;
    if (read <= 0) throw yasl::Exception("OS random source failed");
    done += static_cast<size_t>(read);
  }
  return value;
}
yasl::Vector<Pair> symmetric(yasl::Span<const Pair> pairs) {
  yasl::Vector<Pair> result;
  if (pairs.size() > result.max_size() / 2)
    throw yasl::LengthError("pairs");
  result.reserve(pairs.size() * 2);
  for (const auto pair : pairs) {
    result.push_back(pair);
    result.push_back({pair.receiver, pair.giver});
  }
  return result;
}
bool valid(uint32 n, yasl::Span<const Pair> exclusions,
           yasl::Span<const uint32> receivers) {
  if (n < 2 || receivers.size() != n) return false;
  yasl::Vector<uint8> seen(n, uint8{0});
  for (uint32 i = 0; i < n; ++i) {
    const auto r = receivers[i];
    if (r >= n || r == i || seen[r]) return false;
    seen[r] = 1;
  }
  for (const auto pair : exclusions) {
    if (pair.giver >= n || pair.receiver >= n ||
        receivers[pair.giver] == pair.receiver)
      return false;
  }
  return true;
}
Result draw(uint32 n, yasl::Span<const Pair> exclusions, Budget budget,
            IRandom& random) {
  if (n < 2 || n == yasl::max_value<uint32>())
    return {Status::kInvalidInput, {}, {}};
  for (const auto pair : exclusions)
    if (pair.giver >= n || pair.receiver >= n || pair.giver == pair.receiver)
      return {Status::kInvalidInput, {}, {}};
  // Check before any allocation. Conservative logical working-set estimate;
  // allocator metadata/page rounding is outside this budget.
  const uint64 cells = uint64{n} * n;
  const uint64 overhead = uint64{n} * 64;
  if (cells > yasl::max_value<size_t>() ||
      overhead > budget.max_memory_bytes ||
      cells > budget.max_memory_bytes - overhead)
    return {Status::kResourceLimitExceeded, {}, {}};
  Work work(budget.max_work);
  try {
    work.use(cells);
    work.use(exclusions.size());
    yasl::Vector<uint8> forbidden(static_cast<size_t>(cells), uint8{0});
    for (uint32 i = 0; i < n; ++i) forbidden[size_t{i} * n + i] = 1;
    for (const auto pair : exclusions)
      forbidden[size_t{pair.giver} * n + pair.receiver] = 1;
    // Iterative augmenting paths: no input-dependent native recursion depth.
    yasl::Vector<uint32> left(n, n), right(n, n), parent(n, n), queue(n, n);
    yasl::Vector<uint8> seen_left(n, uint8{0});
    for (uint32 root = 0; root < n; ++root) {
      work.use(uint64{n} * 2);
      for (uint32 i = 0; i < n; ++i) {
        parent[i] = n;
        seen_left[i] = 0;
      }
      size_t head = 0, tail = 1;
      queue[0] = root;
      seen_left[root] = 1;
      uint32 free_receiver = n;
      while (head < tail && free_receiver == n) {
        const auto giver = queue[head++];
        for (uint32 receiver = 0; receiver < n; ++receiver) {
          work.use();
          if (forbidden[size_t{giver} * n + receiver] || parent[receiver] != n)
            continue;
          parent[receiver] = giver;
          if (right[receiver] == n) {
            free_receiver = receiver;
            break;
          }
          const auto next = right[receiver];
          if (!seen_left[next]) {
            seen_left[next] = 1;
            queue[tail++] = next;
          }
        }
      }
      if (free_receiver == n) {
        yasl::Vector<uint32> affected;
        affected.reserve(n);
        for (uint32 i = 0; i < n; ++i)
          if (seen_left[i]) affected.push_back(i);
        return {Status::kUnsatisfiable, {}, yasl::move(affected)};
      }
      while (free_receiver != n) {
        work.use();
        const auto giver = parent[free_receiver];
        const auto previous = left[giver];
        left[giver] = free_receiver;
        right[free_receiver] = giver;
        free_receiver = previous;
      }
    }
    if (budget.max_attempts == 0) return {Status::kSearchLimitExceeded, {}, {}};
    // Dense small games make rejection impractical. Count completions exactly
    // and unrank one uniform integer instead. 20! fits uint64; 21! does not.
    uint64 blocked = 0;
    for (size_t i = 0; i < forbidden.size(); ++i)
      blocked += forbidden[i] == uint8{1};
    if (n <= 20 && blocked - n > uint64{n} * (n - 1) / 4) {
      const uint32 states = uint32{1} << n;
      const uint64 table_bytes = uint64{states} * sizeof(uint64);
      if (table_bytes <= budget.max_memory_bytes - overhead - cells) {
        work.use(states);
        yasl::Vector<uint64> counts(states, uint64{0});
        counts[0] = 1;
        for (uint32 mask = 1; mask < states; ++mask) {
          const uint32 giver = n - yasl::popcount(mask);
          auto remaining = mask;
          while (remaining) {
            work.use();
            const auto receiver =
                static_cast<uint32>(yasl::countr_zero(remaining));
            const auto bit = uint32{1} << receiver;
            if (!forbidden[size_t{giver} * n + receiver])
              counts[mask] += counts[mask ^ bit];
            remaining ^= bit;
          }
        }
        // Reserve the entire worst-case unranking cost BEFORE drawing a rank:
        // a budget cutoff depending on that rank would bias successful results.
        work.use(uint64{n} * n);
        auto rank = bounded(random, counts[states - 1], work);
        auto mask = states - 1;
        yasl::Vector<uint32> receivers(n);
        for (uint32 giver = 0; giver < n; ++giver) {
          for (uint32 receiver = 0; receiver < n; ++receiver) {
            const auto bit = uint32{1} << receiver;
            if (!(mask & bit) || forbidden[size_t{giver} * n + receiver])
              continue;
            const auto completions = counts[mask ^ bit];
            if (rank < completions) {
              receivers[giver] = receiver;
              mask ^= bit;
              break;
            }
            rank -= completions;
          }
        }
        return {Status::kSuccess, yasl::move(receivers), {}, 1};
      }
    }
    yasl::Vector<uint32> permutation(n);
    for (uint64 attempt = 0; attempt < budget.max_attempts; ++attempt) {
      // Always scan the whole permutation: validation work doesn't depend
      // on where a violation occurs. Never return the feasibility matching.
      work.use(uint64{n} * 3);
      for (uint32 i = 0; i < n; ++i) permutation[i] = i;
      for (uint32 i = n - 1; i > 0; --i)
        yasl::swap(permutation[i],
                  permutation[bounded(random, uint64{i} + 1, work)]);
      bool accepted = true;
      for (uint32 i = 0; i < n; ++i)
        accepted &= !forbidden[size_t{i} * n + permutation[i]];
      if (accepted)
        return {Status::kSuccess, yasl::move(permutation), {}, attempt + 1};
    }
    return {Status::kSearchLimitExceeded, {}, {}, budget.max_attempts};
  } catch (const Limit&) {
    return {Status::kSearchLimitExceeded, {}, {}};
  }
}
}  // namespace santa
