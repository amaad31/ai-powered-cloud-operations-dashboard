FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy
RUN go build -o backend-api .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/backend-api .
EXPOSE 8080
CMD ["./backend-api"]
