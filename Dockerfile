FROM golang
RUN go get github.com/mshirley/gobots
ADD cert.pem /app/cert.pem
ADD key.pem /app/key.pem
WORKDIR /app/
EXPOSE 1337