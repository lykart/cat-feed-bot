FROM golang:1.22.5-alpine AS builder
WORKDIR /src/app
COPY . .
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) \
        go build -ldflags="-w -s" -buildvcs=false -o /app main.go

FROM alpine:3.20.3
COPY --from=builder /app /app
RUN apk add --no-cache tzdata
ENTRYPOINT ["/app"]