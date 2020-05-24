FROM golang
RUN mkdir -p /go/src/github.com/mshirley
RUN git clone https://github.com/mshirley/gobots /go/src/github.com/mshirley/gobots
WORKDIR /go/src/github.com/mshirley/gobots/server
RUN go get ./...
RUN go get -u github.com/gobuffalo/packr/v2/packr2
ADD config/config.json config/config.json
ADD pki/cert.pem pki/cert.pem
ADD pki/key.pem pki/key.pem
RUN packr2 build
EXPOSE 1337
CMD ["server"]
