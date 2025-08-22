FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o debserve

FROM alpine:3

WORKDIR /packages

COPY ./as-pwd /usr/local/bin/as-pwd
RUN apk add --no-cache su-exec
COPY --from=builder /app/debserve /usr/local/bin/debserve

ENTRYPOINT ["as-pwd"]
CMD ["debserve", "--watch", "--listen", ":80"]