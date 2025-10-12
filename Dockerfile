# --- Stage 1: Builder ---
# Use a specific Go version for reproducibility
FROM golang:1.24-alpine AS builder

# Arguments we will pass from docker-compose.yml
ARG SERVICE_NAME=payment-gateway
ARG SERVICE_PATH=./cmd/main.go

# Set the working directory
WORKDIR /app

# Install the packages required for assembly and ca-certificates for HTTPS requests
RUN apk add --no-cache git ca-certificates tzdata

# Copy and download dependencies separately to use the Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

COPY cmd/ cmd/
COPY internal/ internal/
COPY configs/ configs/ 

# Проверь, что configs/ теперь в /app/configs
RUN ls -la /app/configs

# Build the application using arguments
#RUN CGO_ENABLED=0 GOOS=linux go build -a -o /app/${SERVICE_NAME} ${SERVICE_PATH}
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /app/app ${SERVICE_PATH}

# --- Stage 2: The Final Look ---
FROM alpine:latest

# Create a group and user without rights
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Switch to this user
USER appuser

# Set the working directory
WORKDIR /home/appuser

# Copy ONLY the compiled binary from the build stage
# and immediately assign it the correct owner
COPY --from=builder /app/configs ./configs

#COPY --from=builder /app/${SERVICE_NAME} .
COPY --from=builder /app/app ./app

# Open the port (informative, real mapping in docker-compose)
EXPOSE 8080

# Launch the application
#CMD ["./${SERVICE_NAME}"]
CMD ["./app"]