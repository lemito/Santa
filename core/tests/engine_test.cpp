#include <santa/abi.h>
#include <santa/engine.h>
#include <algorithm>
#include <chrono>
#include <cstdlib>
#include <iostream>
#include <random>
#include <set>
#include <vector>

#define CHECK(x) do { if (!(x)) { std::cerr << "Failed line " << __LINE__ << ": " #x "\n"; std::exit(1); } } while (false)
struct Random : santa::IRandom {
  std::mt19937_64 generator{42};
  uint64 next() override { return generator(); }
};
struct MaxRandom : santa::IRandom {
  uint64 calls = 0;
  uint64 next() override {
    ++calls;
    return yasl::max_value<uint64>();
  }
};
void valid_rejects_invalid_exclusion() {
  const yasl::Vector<uint32> receivers{1, 0};
  const yasl::Vector<santa::Pair> self{{0, 0}};
  CHECK(!santa::valid(2, self.span(), receivers.span()));

  const yasl::Vector<santa::Pair> out_of_range{{0, 2}};
  CHECK(!santa::valid(2, out_of_range.span(), receivers.span()));
}
void exact_sampler_ignores_density_heuristic() {
  MaxRandom random;
  const yasl::Vector<santa::Pair> exclusions{{0, 1}};
  santa::Budget budget{1, 100000000, 128 * 1024 * 1024};
  const auto result = santa::draw(20, exclusions.span(), budget, random);
  CHECK(result.status == santa::Status::kSuccess);
  CHECK(random.calls == 1);
  CHECK(santa::valid(20, exclusions.span(), result.receivers.span()));
}
constexpr santa::Budget kBudget{100000, 100000000, 128 * 1024 * 1024};
constexpr SantaBudget kAbiBudget{100000, 100000000, 128 * 1024 * 1024};
bool brute(uint32 n, const yasl::Vector<santa::Pair>& pairs) {
  yasl::Vector<uint32> p(n);
  for (uint32 i = 0; i < n; ++i) p[i] = i;
  do { if (santa::valid(n, pairs.span(), p.span())) return true; }
  while (std::next_permutation(p.begin(), p.end()));
  return false;
}
void exhaustive(uint32 n, bool symmetric) {
  Random random;
  yasl::Vector<santa::Pair> edges;
  for (uint32 i = 0; i < n; ++i)
    for (uint32 j = 0; j < n; ++j)
      if (i != j && (!symmetric || i < j)) edges.push_back({i,j});
  for (uint64 mask = 0; mask < (uint64{1} << edges.size()); ++mask) {
    yasl::Vector<santa::Pair> pairs;
    for (size_t i = 0; i < edges.size(); ++i)
      if (mask & (uint64{1} << i)) pairs.push_back(edges[i]);
    if (symmetric) pairs = santa::symmetric(pairs.span());
    const auto exists = brute(n, pairs);
    auto budget = kBudget;
    budget.max_attempts = 0;
    const auto result = santa::draw(n, pairs.span(), budget, random);
    CHECK(result.status == (exists ? santa::Status::kSearchLimitExceeded : santa::Status::kUnsatisfiable));
    if (!exists) {
      std::set<uint32> neighbors;
      CHECK(!result.affected_givers.empty());
      for (auto giver : result.affected_givers)
        for (uint32 r = 0; r < n; ++r) {
          bool allowed = giver != r;
          for (auto p : pairs) if (p.giver == giver && p.receiver == r) allowed = false;
          if (allowed) neighbors.insert(r);
        }
      CHECK(neighbors.size() < result.affected_givers.size());
    } else {
      const auto drawn = santa::draw(n, pairs.span(), kBudget, random);
      CHECK(drawn.status == santa::Status::kSuccess);
      CHECK(santa::valid(n, pairs.span(), drawn.receivers.span()));
    }
  }
}
// Enumerate every Fisher-Yates choice sequence for n=4. Each permutation,
// including the allowed two-cycle example, occurs exactly once.
struct Choices : santa::IRandom {
  uint64 values[3];
  size_t pos = 0;
  uint64 next() override { return values[pos++]; }
};
int main() {
  Random random;
  valid_rejects_invalid_exclusion();
  exact_sampler_ignores_density_heuristic();
  for (auto n : {0u, 1u}) CHECK(santa::draw(n, {}, kBudget, random).status == santa::Status::kInvalidInput);
  CHECK(santa::draw(2, {}, kBudget, random).status == santa::Status::kSuccess);
  CHECK(santa::draw(3, {}, kBudget, random).status == santa::Status::kSuccess);
  CHECK(santa::draw(5, {}, kBudget, random).status == santa::Status::kSuccess);
  yasl::Vector<santa::Pair> duplicate{{0,1},{0,1}};
  CHECK(santa::draw(3, duplicate.span(), kBudget, random).status == santa::Status::kSuccess);
  yasl::Vector<santa::Pair> self{{1,1}};
  CHECK(santa::draw(3, self.span(), kBudget, random).status == santa::Status::kInvalidInput);
  yasl::Vector<santa::Pair> directed{{0,1}};
  auto directed_result = santa::draw(3, directed.span(), kBudget, random);
  CHECK(directed_result.status == santa::Status::kSuccess);
  CHECK(santa::valid(3, directed.span(), directed_result.receivers.span()));
  CHECK(directed_result.receivers[0] != 1);
  yasl::Vector<santa::Pair> bad{{0,4}};
  CHECK(santa::draw(4, bad.span(), kBudget, random).status == santa::Status::kInvalidInput);
  auto tiny = kBudget; tiny.max_memory_bytes = 1;
  CHECK(santa::draw(4, {}, tiny, random).status == santa::Status::kResourceLimitExceeded);
  tiny = kBudget; tiny.max_work = 0;
  CHECK(santa::draw(4, {}, tiny, random).status == santa::Status::kSearchLimitExceeded);
  for (uint32 n = 2; n <= 4; ++n) exhaustive(n, false);
  exhaustive(5, true);
  const yasl::Vector<santa::Pair> pair{{0,1}};
  const auto exclusions = santa::symmetric(pair.span());
  const yasl::Vector<uint32> example{2,3,0,1};
  CHECK(santa::valid(4, exclusions.span(), example.span()));
  // ABI boundary, untouched output on errors, duplicates and directed semantics.
  uint32 out[]{99,99,99,99};
  CHECK(santa_abi_version() == SANTA_ABI_VERSION);
  CHECK(santa_draw(4, nullptr, 1, kAbiBudget, out, 4) == SANTA_INVALID_INPUT);
  CHECK(santa_draw(4, nullptr, 0, kAbiBudget, out, 3) == SANTA_INVALID_INPUT);
  CHECK(out[0] == 99);
  {
    yasl::mem::ScopedDefault fail_allocations(yasl::mem::null_resource());
    CHECK(santa_draw(4, nullptr, 0, kAbiBudget, out, 4) == SANTA_RESOURCE_LIMIT_EXCEEDED);
    CHECK(out[0] == 99);
  }
  CHECK(santa_draw(4, nullptr, 0, kAbiBudget, out, 4) == SANTA_SUCCESS);
  for (uint32 n : {10u, 100u, 1000u}) {
    auto start = std::chrono::steady_clock::now();
    const auto result = santa::draw(n, {}, kBudget, random);
    CHECK(result.status == santa::Status::kSuccess);
    CHECK(santa::valid(n, {}, result.receivers.span()));
    std::cout << "n=" << n << " attempts=" << result.attempts << " us="
      << std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::steady_clock::now()-start).count() << '\n';
  }
  // A feasible unique directed cycle: exact counting avoids rejection starvation.
  yasl::Vector<santa::Pair> dense;
  for (uint32 i = 0; i < 10; ++i) for (uint32 j = 0; j < 10; ++j)
    if (j != i && j != (i + 1) % 10) dense.push_back({i,j});
  tiny = kBudget; tiny.max_attempts = 1;
  auto dense_result = santa::draw(10, dense.span(), tiny, random);
  CHECK(dense_result.status == santa::Status::kSuccess);
  CHECK(santa::valid(10,dense.span(),dense_result.receivers.span()));
  // Exhaustively unrank every possible result for dense directed graphs.
  for (uint64 mask = 0; mask < (uint64{1} << 12); ++mask) {
    yasl::Vector<santa::Pair> rules;
    uint32 edge = 0;
    for (uint32 i = 0; i < 4; ++i) for (uint32 j = 0; j < 4; ++j)
      if (i != j) { if (mask & (uint64{1} << edge)) rules.push_back({i,j}); ++edge; }
    if (rules.size() <= 3) continue;
    std::set<std::vector<uint32>> expected, observed;
    yasl::Vector<uint32> candidate{0,1,2,3};
    do { if (santa::valid(4,rules.span(),candidate.span())) expected.insert({candidate.begin(),candidate.end()}); }
    while (std::next_permutation(candidate.begin(),candidate.end()));
    for (uint64 rank = 0; rank < expected.size(); ++rank) {
      Choices source; source.values[0] = expected.size() + rank;
      auto unranked = santa::draw(4,rules.span(),kBudget,source);
      CHECK(unranked.status == santa::Status::kSuccess && source.pos == 1);
      CHECK(observed.insert({unranked.receivers.begin(),unranked.receivers.end()}).second);
    }
    CHECK(expected == observed);
  }
  tiny = kBudget; tiny.max_memory_bytes = UINT64_MAX;
  CHECK(santa::draw(UINT32_MAX - 1, {}, tiny, random).status == santa::Status::kResourceLimitExceeded);
  std::cout << "Exhaustive graphs, Hall witnesses, shuffle traces, invariants and ABI passed\n";
}
