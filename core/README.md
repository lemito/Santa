# Native algorithm notes

The Santa core solves a perfect matching problem for a bipartite graph: each left vertex is a giver and each right vertex is a receiver. A valid assignment is a perfect matching that also excludes every diagonal edge and every configured directed exclusion.

The matching pass is only an existence check. Its traversal order must not be used as a probability distribution because randomized matching is not generally uniform over all perfect matchings.

For small inputs (`n <= 20`) the engine counts completions with subset dynamic programming. If `count[mask]` is the number of valid completions using receiver bits in `mask`, then the next giver is `n - popcount(mask)` and:

`count[mask] = sum(count[mask without receiver])` over allowed receivers in `mask`.

A uniformly sampled rank in `[0, count[full])` is unranked from left to right. Every valid permutation corresponds to exactly one rank, so the result is uniform when the count is represented exactly. The current counter is `uint64_t`; `20!` fits, while `21!` does not. The implementation must not silently extend this arithmetic beyond its proven range.

For larger inputs the engine samples a uniformly distributed permutation with Fisher–Yates. Each bounded random index uses rejection before modulo, so modulo bias is not introduced. The complete permutation is checked before acceptance; there is no early result-dependent validation shortcut and no fallback to the feasibility matching.

A search or resource budget is an operational cutoff. Reaching it returns `SearchLimitExceeded` or `ResourceLimitExceeded`. It never changes the distribution of a result that was accepted and never becomes `Unsatisfiable`.

The C++ tests compare existence against exhaustive permutation enumeration for small graphs and enumerate exact ranks for dense small graphs. Statistical tests may detect regressions but cannot replace the recurrence and unranking argument above.
