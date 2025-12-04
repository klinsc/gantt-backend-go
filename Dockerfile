FROM golang:1.20-bullseye AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /src/gantt-backend-go ./...

FROM debian:12-slim

WORKDIR /app
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates tzdata \
	&& rm -rf /var/lib/apt/lists/*

COPY demodata ./demodata
COPY config.yml ./config.yml
COPY --from=builder /src/gantt-backend-go ./gantt-backend-go

EXPOSE 8080

CMD ["/app/gantt-backend-go"]