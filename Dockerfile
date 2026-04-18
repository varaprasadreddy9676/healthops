# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
WORKDIR /app/backend
RUN CGO_ENABLED=0 go build -o healthmon ./cmd/healthmon

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/backend/healthmon .
COPY --from=builder /app/backend/config ./config/

# Create data directory
RUN mkdir -p data

ENV CONFIG_PATH=/root/config/default.json
ENV STATE_PATH=/root/data/state.json

EXPOSE 8080

CMD ["./healthmon"]
