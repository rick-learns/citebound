# 0012. Ship a distroless container image with the corpus and recordings

- Status: Accepted
- Date: 2026-11-17

## Context

Running Citebound needs Go and Node.js to build it, and reviewers trying the
project may have neither. Deploying it needs an artifact that starts the same
way everywhere. The server already embeds the UI
([decision 0009](0009-embed-the-web-ui.md)) and reads its corpus from a
directory, so one binary and two data directories are all it needs.

## Decision

- **Three build stages.** Node builds the UI, Go builds a static binary with
  it embedded (`CGO_ENABLED=0`, which the pure-Go SQLite allows), and the
  result runs on `gcr.io/distroless/static-debian12:nonroot`: no shell, no
  package manager, and a non-root user.
- **Pinned bases.** Each base image is pinned by its multi-platform index
  digest, so a rebuild uses the same bases until the digests are changed on
  purpose.
- **Data in the image.** The pinned corpus and `evals/recordings/` are
  copied in, so the image answers, extractively or by replay, with no network
  and no key. Recordings add about 1.2 MB.
- **Listen on every interface by default.** The image's command is
  `serve -addr 0.0.0.0:8080`, since 127.0.0.1 inside a container cannot be
  reached from the host. The binary's own default stays 127.0.0.1.
- **Keys at run time only.** Provider keys are passed with `-e`, never as
  build arguments, which would be stored in the image's history.
- **CI builds and smoke-tests the image on every pull request**
  (`scripts/container-smoke.sh`), and `make check` runs the same script.

## Consequences

- Trying the UI needs only Docker or Podman.
- `make check` now needs Docker or Podman as well.
- Pinned digests do not pick up base-image security fixes by themselves.
  They have to be bumped, by hand or by a dependency bot.
- No image is published to a registry yet. Users build it themselves.

## Alternatives considered

- **Alpine or Debian slim as the runtime base.** They add a shell and a
  package manager that the server never uses, and more packages to patch.
- **`scratch`.** It has no CA certificates or `/etc/passwd`, which a model
  call over TLS and a named non-root user need. Distroless static has both.
- **Mounting the corpus at run time.** It keeps the image smaller, but the
  image would not run on its own, and a mounted corpus could differ from the
  pinned one the binary was tested with.
