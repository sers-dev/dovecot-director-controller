FROM golang:1.15.3-alpine3.12

WORKDIR /workdir/go

ADD cmd/main.go go.mod ./

RUN go get -d -v ./... && go install -v ./... && go build main.go

CMD ["main"]