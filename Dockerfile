FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bot ./cmd/bot/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bot /bot
COPY migrations/ /migrations/

ENTRYPOINT ["/bot"]
