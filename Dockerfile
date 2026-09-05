# Build the bot as a static binary, then ship it on a minimal base.
FROM docker.io/library/golang:1.26-alpine AS build

ARG VERSION=dev

WORKDIR /src

# Dependencies are their own layer: they only change when go.mod/go.sum do.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

# CGO off keeps the binary self contained, so the runtime image needs no libc
# beyond what Alpine already has.
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags "-s -w -X mitoboat/internal/bot.Version=${VERSION}" \
      -o /out/mitoboat ./cmd/mitoboat

FROM docker.io/library/alpine:3.22

# Twitch is reached over HTTPS, so the image needs root certificates.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 mitoboat

COPY --from=build /out/mitoboat /usr/local/bin/mitoboat

USER mitoboat
EXPOSE 8080

# No flags: serve chat and the authorization flow. Override the command with
# -s to migrate, or -a to run only the authorization server.
ENTRYPOINT ["/usr/local/bin/mitoboat"]
