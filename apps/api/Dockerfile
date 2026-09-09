# syntax=docker/dockerfile:1

FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/synvideo-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/synvideo-migrate ./cmd/migrate

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
  && rm -rf /var/lib/apt/lists/*
RUN useradd --system --home /nonexistent --shell /usr/sbin/nologin synvideo
COPY --from=build /out/synvideo-api /usr/local/bin/synvideo-api
COPY --from=build /out/synvideo-migrate /usr/local/bin/synvideo-migrate
USER synvideo
EXPOSE 10000
ENTRYPOINT ["/usr/local/bin/synvideo-api"]
