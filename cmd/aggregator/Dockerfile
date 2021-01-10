FROM golang:1.14-stretch as build
ADD . /src/aggregator
WORKDIR /src/aggregator
RUN go build -o /bin/aggregator ./cmd/aggregator

FROM ubuntu:16.04
COPY --from=build /bin/aggregator  /app/
ENTRYPOINT ["/app/aggregator"]

