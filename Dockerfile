FROM golang:alpine

RUN apk update && apk add git build-base ffmpeg

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY static/ static/
COPY cmd/ cmd/
COPY *.go ./
COPY index.html .
RUN go mod tidy
RUN go build cmd/godm.go

EXPOSE 8080/tcp
CMD ["/app/godm", "server", "--address=:8080", "/app/output"]