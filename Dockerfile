FROM golang:alpine

RUN apk update && apk add git
RUN git clone https://github.com/micahjmartin/godm.git /app
WORKDIR /app

RUN go mod tidy
RUN go build cmd/godm.go

EXPOSE 8080/tcp
CMD ["/app/godm", "server", "--address=':8080'", "/app/output"]