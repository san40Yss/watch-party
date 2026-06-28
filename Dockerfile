FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o watchparty ./cmd/server

FROM alpine:3.20
# ffmpeg included now — needed from Step 2 (ffprobe + transcode)
RUN apk add --no-cache ffmpeg ca-certificates
WORKDIR /app
COPY --from=builder /app/watchparty .
EXPOSE 8080
CMD ["./watchparty"]
