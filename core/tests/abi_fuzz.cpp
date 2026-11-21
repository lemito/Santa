#include <santa/abi.h>

#include <cstdint>
#include <random>
#include <vector>

int main() {
  std::mt19937_64 random(0x53414e5441ULL);
  SantaBudget budget{10000, 1000000, 16 * 1024 * 1024};
  for (unsigned trial = 0; trial != 2000; ++trial) {
    const uint32_t n = static_cast<uint32_t>(random() % 9);
    const uint64_t count = random() % 25;
    std::vector<SantaPair> pairs(count);
    for (auto& pair : pairs) {
      pair.giver = static_cast<uint32_t>(random() % (n + 3));
      pair.receiver = static_cast<uint32_t>(random() % (n + 3));
    }
    std::vector<uint32_t> output(n ? n : 1, 0xfeedfaceU);
    const int32_t status = santa_draw(n, pairs.empty() ? nullptr : pairs.data(), count,
                                     budget, output.data(), n);
    if (status == SANTA_SUCCESS) {
      std::vector<bool> seen(n, false);
      for (uint32_t receiver : output) {
        if (receiver >= n || seen[receiver])
          return 1;
        seen[receiver] = true;
      }
    } else {
      for (uint32_t value : output)
        if (value != 0xfeedfaceU)
          return 2;
    }
  }
  return 0;
}
