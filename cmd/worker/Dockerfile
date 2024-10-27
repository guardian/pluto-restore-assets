# To build:
# docker build -f worker/Dockerfile -t guardianmultimedia/pluto-project-restore-worker:DEV .

FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o restore-worker ./cmd/worker

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/restore-worker .

CMD ["./restore-worker"]
