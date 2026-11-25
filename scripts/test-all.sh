#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go_cmd="${GO:-$(command -v go)}"
if [[ "$go_cmd" == /snap/bin/go && -x /snap/go/current/bin/go ]]; then go_cmd=/snap/go/current/bin/go; fi

cmake -S . -B build -DBUILD_TESTING=ON -DCMAKE_BUILD_TYPE=Release
cmake --build build -j2
ctest --test-dir build --output-on-failure
native_prefix="$PWD/build-native-prefix"
rm -rf "$native_prefix"
cmake --install build --prefix "$native_prefix"

if command -v go >/dev/null 2>&1; then
  (cd go && PKG_CONFIG_PATH="$native_prefix/lib/pkgconfig" LD_LIBRARY_PATH="$native_prefix/lib" "$go_cmd" test ./...)
  (cd go && PKG_CONFIG_PATH="$native_prefix/lib/pkgconfig" LD_LIBRARY_PATH="$native_prefix/lib" "$go_cmd" test -race ./...)
fi

if command -v dotnet >/dev/null 2>&1; then
  dotnet restore santa/santa.csproj --locked-mode
  dotnet build santa --no-restore
fi
