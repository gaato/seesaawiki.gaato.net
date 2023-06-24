FROM golang:1.19-alpine

WORKDIR /app

COPY src .

RUN go mod download

RUN go build -o /main

EXPOSE 1323

CMD ["/main"]