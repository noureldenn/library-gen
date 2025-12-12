
FROM golang:1.25-alpine AS builder


WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -o library-api cmd/server/main.go


FROM alpine:latest


WORKDIR /root/

COPY --from=builder /app/library-api .


EXPOSE 9000


CMD ["./library-api"]