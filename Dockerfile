# Citebound with the web UI embedded. Base images are pinned by digest.
#
#   podman build --build-arg VERSION=$(git describe --tags --always) -t citebound .
#   podman run --rm -p 8080:8080 citebound
#
# The image holds the pinned corpus and the recorded model answers, so it
# runs with no API key and no network. See docs/container.md.

FROM docker.io/library/node:24-alpine@sha256:116b800c3754b440ee0aa3ed2f20f5bf3d99a554bfd9adeea09ea8017f6e85f5 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM docker.io/library/golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS go
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/*.go web/
COPY --from=web /src/web/dist web/dist
RUN CGO_ENABLED=0 go build -tags embedui -trimpath \
	-ldflags "-s -w -X github.com/rick-learns/citebound/internal/version.Version=${VERSION}" \
	-o /out/citebound ./cmd/citebound

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /app
COPY --from=go /out/citebound /app/citebound
COPY corpus/ /app/corpus/
COPY evals/recordings/ /app/evals/recordings/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/citebound"]
# Listen on every interface: inside a container, 127.0.0.1 is unreachable
# from the host.
CMD ["serve", "-addr", "0.0.0.0:8080"]
