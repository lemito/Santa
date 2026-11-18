#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j 2
ctest --test-dir build --output-on-failure
(cd go && LD_LIBRARY_PATH=../build go test ./...)
dotnet restore santa/santa.csproj --locked-mode
dotnet build santa --no-restore
: "${SANTA_TEST_DB:?Point SANTA_TEST_DB to a disposable PostgreSQL database}"
dotnet run --project tests/Santa.Tests
