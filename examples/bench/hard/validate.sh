#!/usr/bin/env bash
# Validate every reference implementation in the HARD benchmark tier. These
# problems target reader-invisible defect classes (overflow, aliasing, Unicode,
# concurrency), so the reference tests must actually catch the trap. Each must
# compile and pass its own tests (plain, and under -race for conc_ problems).
# Reference impls are the ground-truth labels the judge-vs-execution comparison
# depends on, so they are verified by execution here rather than trusted.
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
    echo "FAIL  $id (test)"
    echo "$out" | sed 's/^/      /'
    fail=$((fail + 1))
    continue
  fi
  if [ "${id#hard_conc_}" != "$id" ]; then
    rout=$(cd "$dir" && go test -race -count=5 ./... 2>&1)
    if [ $? -ne 0 ]; then
      echo "FAIL  $id (race)"
      echo "$rout" | sed 's/^/      /'
      fail=$((fail + 1))
      continue
    fi
    echo "ok    $id (test + race)"
  else
    echo "ok    $id"
  fi
done

echo "----"
echo "$((total - fail))/$total hard reference implementations pass"
[ "$fail" -eq 0 ]
