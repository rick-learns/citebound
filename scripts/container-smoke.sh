#!/bin/sh
# Build the container image and check that it serves the UI and answers, in
# extractive and replay mode, with no API key and no network. CI runs this.
# Uses docker, or podman if docker is missing.
set -eu

engine=${CONTAINER_ENGINE:-$(command -v docker || command -v podman)}
image=citebound:smoke
port=${SMOKE_PORT:-18400}
name=citebound-smoke-$$
model=openrouter/nvidia/nemotron-3-ultra-550b-a55b:free

"$engine" build -q --build-arg VERSION=smoke -t "$image" . >/dev/null
trap '"$engine" rm -f "$name" >/dev/null 2>&1 || true' EXIT

# expect <text> <command...>: run the command, which must succeed, and check
# its standard output. Standard error is shown on failure but never matched.
expect() {
	want=$1
	shift
	err=$(mktemp)
	if ! out=$("$@" 2>"$err"); then
		printf 'container-smoke: failed: %s\n' "$*" >&2
		cat "$err" >&2
		rm -f "$err"
		exit 1
	fi
	rm -f "$err"
	case $out in
	*"$want"*) printf 'ok: %s\n' "$want" ;;
	*)
		printf 'container-smoke: expected %s, got:\n%s\n' "$want" "$out" >&2
		exit 1
		;;
	esac
}

# serve <args...>: start the server and wait until it answers.
serve() {
	"$engine" rm -f "$name" >/dev/null 2>&1 || true
	"$engine" run -d --name "$name" -p "127.0.0.1:$port:8080" "$image" "$@" >/dev/null
	i=0
	until curl -fsS "http://127.0.0.1:$port/api/meta" >/dev/null 2>&1; do
		i=$((i + 1))
		if [ "$i" -ge 30 ]; then
			"$engine" logs "$name" >&2
			printf 'container-smoke: the server did not start\n' >&2
			exit 1
		fi
		sleep 1
	done
}

ask() {
	curl -fsSN -X POST -H 'Content-Type: application/json' \
		-d '{"question": "Can a refresh token be bound to a client?"}' \
		"http://127.0.0.1:$port/api/ask"
}

expect smoke "$engine" run --rm "$image" version
expect '"user":"nonroot' "$engine" image inspect "$image" --format '{"user":"{{.Config.User}}"}'

serve # the image's default command
expect '"mode":"extractive"' curl -fsS "http://127.0.0.1:$port/api/meta"
expect '<title>Citebound' curl -fsS "http://127.0.0.1:$port/"
expect '"mode":"extractive"' ask

serve serve -addr 0.0.0.0:8080 -model "$model" -replay evals/recordings/nemotron-3-ultra.jsonl
expect '"verified":3' ask
