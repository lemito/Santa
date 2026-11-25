#include <santa/engine.h>

#include <cerrno>
#include <sys/random.h>

#include <yasl/core/exception.h>

namespace santa {
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
}  // namespace santa
