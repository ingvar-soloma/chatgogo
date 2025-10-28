FROM golang:1.25.3-alpine3.22 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/chatgogo-backend ./cmd/main.go

FROM alpine:3.22.2
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/chatgogo-backend .
COPY --from=builder /app/.env .

EXPOSE 8080

CMD ["./chatgogo-backend"]