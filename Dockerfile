FROM golang
RUN go get -u github.com/mshirley/gobots
ADD cert.pem /app/cert.pem
ADD key.pem /app/key.pem
