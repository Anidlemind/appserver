FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/carstats-api ./src/api

FROM alpine:3.20

RUN adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /bin/carstats-api /usr/local/bin/carstats-api

USER appuser

EXPOSE 50051

ENTRYPOINT ["/usr/local/bin/carstats-api"]
