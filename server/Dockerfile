FROM golang
RUN git clone github.com/mshirley/gobots
RUN cd gobots/server
RUN packr2 build
ADD cert.pem pki/cert.pem
ADD key.pem pki/key.pem
EXPOSE 1337
CMD ["server"]