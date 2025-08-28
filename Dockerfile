FROM golang:1.25-alpine AS build

WORKDIR /app

ARG version="undefined"

COPY . .

RUN go build -ldflags "-X main.version=${version} -s -w"  -o ssqueue ./cmd/main

FROM alpine:latest

WORKDIR /root/

EXPOSE 8080
EXPOSE 8081

COPY --from=build /app/ssqueue .

CMD ["./ssqueue"]
