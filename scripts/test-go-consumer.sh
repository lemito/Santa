#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
prefix=${1:-"$root/build-native-prefix"}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cat >"$work/go.mod" <<EOF
module consumer

go 1.23

require (
	github.com/lemito/Santa/go v0.0.0
	github.com/google/uuid v1.6.0
)

replace github.com/lemito/Santa/go => $root/go
EOF
cp "$root/go/go.sum" "$work/go.sum"
mkdir -p "$work/cmd/consumer"
cat >"$work/cmd/consumer/main.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/lemito/Santa/go/santa"
)

func main() {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	result, err := santa.Draw(context.Background(), santa.Input{Participants: ids}, santa.Options{})
	if err != nil || result.Status != santa.Success || len(result.Assignments) != len(ids) {
		fmt.Fprintf(os.Stderr, "draw failed: status=%v err=%v assignments=%d\n", result.Status, err, len(result.Assignments))
		os.Exit(1)
	}
	fmt.Println("external Go consumer: PASS")
}
EOF

(cd "$work" && GOSUMDB=off GOPROXY=off PKG_CONFIG_PATH="$prefix/lib/pkgconfig" LD_LIBRARY_PATH="$prefix/lib" go run -mod=mod ./cmd/consumer)
