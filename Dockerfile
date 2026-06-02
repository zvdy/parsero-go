# Multi-stage build. By default builds the server (parserod); pass
#   --build-arg TARGET=parsero
# to build the CLI instead.
FROM golang:1.25-alpine AS build

ARG TARGET=parserod
WORKDIR /app

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build.
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
WORKDIR /home/app
COPY --from=build /out/app /usr/local/bin/app

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/app"]
