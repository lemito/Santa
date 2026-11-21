#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
native_dir="$PWD/build"
go_cmd="${GO:-$(command -v go)}"
if [[ "$go_cmd" == /snap/bin/go && -x /snap/go/current/bin/go ]]; then go_cmd=/snap/go/current/bin/go; fi

cmake -S . -B build -DBUILD_TESTING=ON -DCMAKE_BUILD_TYPE=Release
cmake --build build -j2
ctest --test-dir build --output-on-failure

if command -v go >/dev/null 2>&1; then
  (cd go && LD_LIBRARY_PATH="$native_dir" "$go_cmd" test ./...)
  (cd go && LD_LIBRARY_PATH="$native_dir" "$go_cmd" test -race ./...)
fi

if command -v dotnet >/dev/null 2>&1; then
  dotnet restore santa/santa.csproj --locked-mode
  dotnet build santa --no-restore
fi
