FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /llmrouter ./cmd/server/main.go

FROM alpine:latest

COPY --from=builder /llmrouter /llmrouter

EXPOSE 8080

CMD ["/llmrouter"]
