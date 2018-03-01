FROM golang:1.9 AS builder
WORKDIR /go/src/github.com/pusher/quack
COPY . .
RUN go get -d -v; \
    CGO_ENABLED=0 GOOS=linux go build -o /bin/quack github.com/pusher/quack/cmd/quack

FROM scratch
COPY --from=builder /bin/quack /bin/quack

ENTRYPOINT ["/bin/quack"]
