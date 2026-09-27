FROM golang:1.25.6 AS builder

WORKDIR /app


COPY go.mod go.sum ./


RUN go mod download


COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app ./cmd/app/main.go





FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root/

COPY --from=builder /app/app .

CMD ["./app"]