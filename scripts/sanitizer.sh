#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

cmake -S . -B build-sanitize -DBUILD_TESTING=ON -DCMAKE_BUILD_TYPE=Debug -DSANTA_SANITIZE=ON
cmake --build build-sanitize -j2
# LeakSanitizer is enabled explicitly because some sandboxes run tests under ptrace.
ASAN_OPTIONS="${ASAN_OPTIONS:-detect_leaks=${SANTA_DETECT_LEAKS:-0}}" ctest --test-dir build-sanitize --output-on-failure
