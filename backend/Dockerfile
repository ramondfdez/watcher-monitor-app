# Build stage
FROM public.ecr.aws/docker/library/golang:alpine AS builder

WORKDIR /app

# Install git (needed for go mod sometimes)
RUN apk add --no-cache git

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Runtime stage
FROM public.ecr.aws/docker/library/alpine:3.20

# Install only what's needed
RUN apk add --no-cache ca-certificates docker-cli procps coreutils

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
