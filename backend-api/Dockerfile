FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go build -o backend-api .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/backend-api .
EXPOSE 8080
CMD ["./backend-api"]
