# To build:
# docker build -f worker/Dockerfile -t guardianmultimedia/pluto-project-restore-worker:DEV .

FROM golang:1.23 AS builder

WORKDIR /app

COPY .. .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o restore-worker ./worker

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/restore-worker .

CMD ["./restore-worker"]
