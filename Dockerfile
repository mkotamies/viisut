FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o viisut .

FROM alpine:latest
RUN apk add --no-cache libc6-compat

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /app/viisut .

EXPOSE 9000

USER appuser

CMD ["./viisut"]
