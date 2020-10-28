FROM golang:1.15.3-alpine3.12 as builder

WORKDIR /workdir/go

ADD cmd/main.go go.mod ./

RUN go get -d -v ./...
RUN go install -v ./...
RUN go build -o dovecot-director-controller main.go


FROM alpine:3.12

COPY --from=builder /workdir/go/dovecot-director-controller /usr/local/bin/dovecot-director-controller

CMD ["/usr/local/bin/dovecot-director-controller"]
