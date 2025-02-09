FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /mqtt-influxdb-bridge

FROM alpine:3.19

RUN addgroup -S nonroot && adduser -S nonroot -G nonroot

WORKDIR /

COPY --from=builder /mqtt-influxdb-bridge /mqtt-influxdb-bridge

RUN chown nonroot:nonroot /mqtt-influxdb-bridge

USER nonroot:nonroot

CMD ["./mqtt-influxdb-bridge"]
