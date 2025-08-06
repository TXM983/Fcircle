FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN apk add --no-cache git ca-certificates
RUN GOPROXY=https://goproxy.cn,direct go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /fcircle-v2 ./cmd/fetch

FROM alpine:3.17

WORKDIR /app

COPY --from=builder /fcircle-v2 /app/
COPY ./config/ /app/config/
COPY ./start.sh /app/start.sh

RUN chmod +x /app/start.sh

CMD ["/app/start.sh"]
