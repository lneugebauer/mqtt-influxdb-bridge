FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o mqtt-influxdb-bridge

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/mqtt-influxdb-bridge .

CMD ["./mqtt-influxdb-bridge"]