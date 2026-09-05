FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/bad-files-checker ./cmd/bad-files-checker

FROM build AS test

RUN apt-get update \
    && apt-get install -y --no-install-recommends p7zip-full unrar-free tar gzip bzip2 xz-utils ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --shell /usr/sbin/nologin checker
ENV GOCACHE=/tmp/go-build
USER checker
RUN go test ./...

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends unzip p7zip-full unrar-free tar gzip bzip2 xz-utils ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/bad-files-checker /usr/local/bin/bad-files-checker
ENTRYPOINT ["bad-files-checker"]
