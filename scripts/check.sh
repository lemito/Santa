#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
native_dir="$PWD/build"
go_cmd="${GO:-$(command -v go)}"
if [[ "$go_cmd" == /snap/bin/go && -x /snap/go/current/bin/go ]]; then go_cmd=/snap/go/current/bin/go; fi
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j 2
ctest --test-dir build --output-on-failure
mkdir -p artifacts/coverage
(cd go && LD_LIBRARY_PATH="$native_dir" "$go_cmd" test ./... -covermode=atomic -coverpkg=./... -coverprofile=../artifacts/coverage/go.coverprofile)
(cd go && "$go_cmd" tool cover -func=../artifacts/coverage/go.coverprofile > ../artifacts/coverage/go.txt)
if [[ "${SANTA_RACE:-0}" == 1 ]]; then
  (cd go && LD_LIBRARY_PATH="$native_dir" "$go_cmd" test -race ./...)
fi
dotnet restore santa/santa.csproj --locked-mode
dotnet build santa --no-restore
: "${SANTA_TEST_DB:?Point SANTA_TEST_DB to a disposable PostgreSQL database}"
dotnet run --project tests/Santa.Tests
