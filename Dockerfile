FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY packages/api-support/go.mod packages/api-support/
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /support ./cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /support /app/support
EXPOSE 9009
ENTRYPOINT ["/app/support"]
