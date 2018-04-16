FROM golang:1.9 AS builder
WORKDIR /go/src/github.com/pusher/quack
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . .
RUN dep ensure --vendor-only
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/quack github.com/pusher/quack/cmd/quack

FROM alpine
COPY --from=builder /bin/quack /bin/quack

ENTRYPOINT ["/bin/quack"]
