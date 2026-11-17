# Container

The [`Dockerfile`](../Dockerfile) builds Citebound with the web UI embedded,
on a distroless base that runs as a non-root user
([decision 0012](decisions/0012-distroless-container-image.md)). The image is
about 19 MB and holds the pinned corpus and the recorded model answers, so it
needs no API key and no network.

```sh
podman build --build-arg VERSION=$(git describe --tags --always) -t citebound .   # or docker
podman run --rm -p 8080:8080 citebound   # extractive answers, http://127.0.0.1:8080
```

The default command is `serve -addr 0.0.0.0:8080`. A command you pass
replaces it, so repeat `-addr 0.0.0.0:8080`: inside a container, the server's
default address, 127.0.0.1, cannot be reached from the host.

## Replay a real model

```sh
podman run --rm -p 8080:8080 citebound serve -addr 0.0.0.0:8080 \
  -model openrouter/nvidia/nemotron-3-ultra-550b-a55b:free \
  -replay evals/recordings/nemotron-3-ultra.jsonl
```

As outside the container, replay answers only the recorded questions.

## Call a model

Pass the provider's key as an environment variable, never as a build
argument, which would store it in the image:

```sh
podman run --rm -p 8080:8080 -e OPENROUTER_API_KEY citebound serve -addr 0.0.0.0:8080 \
  -model openrouter/nvidia/nemotron-3-ultra-550b-a55b:free
```

`ANTHROPIC_API_KEY` and `OPENAI_API_KEY` work the same way. The server's
guards still apply: per-client rate limits, a cap on concurrent asks
([decision 0010](decisions/0010-rate-limit-and-cap-asks.md)), and a daily
token budget, after which answers fall back to extractive
([decision 0011](decisions/0011-daily-token-budget.md)). See
`citebound serve -h` for their flags.

## Behind a reverse proxy

Rate limits are per client address. Behind a proxy, every request comes from
the proxy, so all users would share one limit. Pass the header your proxy
sets with the real client address, and only if the proxy always sets it,
since a client could otherwise send its own:

```sh
citebound serve -addr 0.0.0.0:8080 -client-ip-header Fly-Client-IP
```

## Checks

`make container` builds the image and runs
[`scripts/container-smoke.sh`](../scripts/container-smoke.sh), which CI also
runs. It checks the version, the non-root user, the UI, an extractive answer
from the default command, and a replayed model answer with all three claims
verified.
