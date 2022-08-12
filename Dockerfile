# Build workshare in a stock Go builder container
FROM golang:alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git
WORKDIR  /go/workshare
COPY . /go/workshare
RUN make all

# Pull workshare into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/workshare/bin/workshare /usr/local/bin/
COPY --from=builder /go/workshare/bin/disco /usr/local/bin/

EXPOSE 8669 11235 11235/udp 55555/udp
ENTRYPOINT ["workshare"]