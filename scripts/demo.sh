#!/bin/sh
# A tour of Citebound in four answers, with no API key and no network. Each
# step checks its own output, so CI runs this as a smoke test.
set -eu

model=openrouter/nvidia/nemotron-3-ultra-550b-a55b:free
recordings=evals/recordings/nemotron-3-ultra.jsonl
bin=$(mktemp -d)/citebound
trap 'rm -f "$bin"; rmdir "$(dirname "$bin")"' EXIT
go build -o "$bin" ./cmd/citebound

# step <title> <expected text> <command...>: run, show, and check the output.
step() {
	title=$1 want=$2
	shift 2
	printf '\n\033[1m== %s\033[0m\n$ citebound %s\n\n' "$title" "$*"
	out=$("$bin" "$@" 2>&1) || true
	printf '%s\n' "$out"
	case $out in
	*"$want"*) ;;
	*)
		printf '\ndemo: expected output containing %s\n' "$want" >&2
		exit 1
		;;
	esac
}

step "1. A real model answers, citing three RFCs (Nemotron 3 Ultra, replayed)" "RFC8705 §7.1 ¶1" \
	ask -model "$model" -replay "$recordings" "Can a refresh token be bound to a client?"

step "2. Asked to quote an RFC that does not exist, it abstains" "answerer-abstained" \
	ask -model "$model" -replay "$recordings" "Quote RFC 9999 §3 on OAuth token lifetimes."

step "3. A hand-written answer with one altered quote: the fake is struck" "struck: quote-not-found" \
	check examples/fabricated-quote.txt

step "4. What verification cannot catch: a real quote beside a false claim passes" "1 of 1 claims verified" \
	check examples/false-claim.txt

printf '\nStep 4 is why the evals measure claim support separately. See docs/answer-format.md#what-verification-proves.\n'
