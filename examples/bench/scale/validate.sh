#!/usr/bin/env bash
# Validate every reference implementation in the SCALE benchmark tier. This tier
# exists to raise n toward the sample-size floor for a deployable certified alpha
# (paper eq. 7: alpha=0.05 needs n>=45), so the problems are deliberately
# tractable and stdlib-only. Each reference must compile and pass its own tests.
# References are the ground-truth labels the certification comparison depends on,
# so they are verified by execution here rather than trusted.
set -u
cd "$(dirname "$0")/refs" || exit 2

# gofmt covers the whole tree because the repo's CI runs `gofmt -l .`.
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "FAIL  gofmt needed:"
  echo "$unformatted" | sed 's/^/      /'
  exit 1
fi

fail=0
total=0
for dir in */; do
  id="${dir%/}"
  [ -f "$dir/solution.go" ] || continue
  total=$((total + 1))
  out=$(cd "$dir" && go test ./... -count=1 2>&1)
  if [ $? -ne 0 ]; then
    echo "FAIL  $id"
    echo "$out" | sed 's/^/      /'
    fail=$((fail + 1))
    continue
  fi
  echo "ok    $id"
done

echo "----"
echo "$((total - fail))/$total scale reference implementations pass"
[ "$fail" -eq 0 ]
