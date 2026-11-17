#!/usr/bin/env bash
# Checks that every commit subject in a range follows Conventional Commits.
# Usage: scripts/check-commits.sh <base-ref> [head-ref]
set -euo pipefail

base=${1:?usage: check-commits.sh <base-ref> [head-ref]}
head=${2:-HEAD}
pattern='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test|eval)(\([a-z0-9-]+\))?!?: [^ ].{0,99}$'

# Capture first so a bad ref fails the script instead of looking like an empty range.
log=$(git log --no-merges --format='%h %s' "${base}..${head}")

status=0
while IFS= read -r line; do
  [[ -z $line ]] && continue
  sha=${line%% *}
  subject=${line#* }
  if [[ ! $subject =~ $pattern ]]; then
    echo "::error::commit ${sha} does not follow Conventional Commits: ${subject}"
    status=1
  fi
done <<< "$log"

if [[ $status -eq 0 ]]; then
  echo "All commit subjects follow Conventional Commits."
fi
exit $status
