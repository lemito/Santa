#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
native_dir="$PWD/build-coverage"
go_cmd="${GO:-$(command -v go)}"
if [[ "$go_cmd" == /snap/bin/go && -x /snap/go/current/bin/go ]]; then go_cmd=/snap/go/current/bin/go; fi

artifact_dir="${SANTA_ARTIFACT_DIR:-artifacts/coverage}"
mkdir -p "$artifact_dir"

cmake -S . -B build-coverage -DBUILD_TESTING=ON -DCMAKE_BUILD_TYPE=Debug -DSANTA_COVERAGE=ON
cmake --build build-coverage -j2
ctest --test-dir build-coverage --output-on-failure
if command -v gcov >/dev/null 2>&1; then
  gcda_files=$(find build-coverage/CMakeFiles -path '*santa*' -name '*.gcda' -print)
  [[ -n "$gcda_files" ]] && gcov -b -c $gcda_files > "$artifact_dir/cpp.txt"
elif command -v llvm-cov >/dev/null 2>&1; then
  llvm-cov gcov build-coverage/CMakeFiles/santa_core.dir/core/src/engine.cpp.gcda > "$artifact_dir/cpp.txt" || true
fi

if command -v go >/dev/null 2>&1; then
  (cd go && LD_LIBRARY_PATH="$native_dir" "$go_cmd" test ./... -covermode=atomic -coverpkg=./... -coverprofile="../$artifact_dir/go.coverprofile")
  (cd go && "$go_cmd" tool cover -func="../$artifact_dir/go.coverprofile" > "../$artifact_dir/go.txt")
fi

if command -v dotnet >/dev/null 2>&1; then
  dotnet build santa --no-restore
fi
