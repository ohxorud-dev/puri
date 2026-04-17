FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
COPY gen/ gen/
COPY services/submission/ services/submission/
WORKDIR /src/services/submission
RUN go mod download
RUN CGO_ENABLED=0 go build -o /submission .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /submission /submission
EXPOSE 8081
CMD ["/submission"]
