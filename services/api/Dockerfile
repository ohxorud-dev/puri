FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
COPY gen/go/ gen/go/
COPY services/api/ services/api/
WORKDIR /src/services/api
RUN go mod download
RUN CGO_ENABLED=0 go build -o /api .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /api /api
COPY db/migrations /db/migrations
EXPOSE 8080
CMD ["/api"]
