FROM golang:1.16-alpine AS build-env
RUN apk add --no-cache gcc musl-dev krb5 krb5-dev krb5-libs krb5-server
WORKDIR /go/src/github.com/fairwindsops/polaris/

ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64


COPY go.mod .
COPY go.sum .
RUN go mod download
RUN go get -u github.com/gobuffalo/packr/v2/packr2

COPY . .
RUN packr2 build -a -o polaris *.go

FROM alpine:3.14
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates
RUN apk add --no-cache gcc musl-dev krb5 krb5-dev krb5-libs  krb5-server

RUN mkdir -p /etc/krb5/conf 
RUN mkdir -p /etc/krb5/k5login
RUN chmod -R a+rx /etc/krb5/conf
RUN chmod -R a+rx /etc/krb5/k5login
RUN addgroup -S polaris && adduser -u 1200 -S polaris -G polaris
USER 1200
COPY --from=build-env /go/src/github.com/fairwindsops/polaris/polaris .

WORKDIR /opt/app

CMD ["polaris"]
