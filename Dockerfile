FROM golang:1.13.4-stretch as build

WORKDIR apps/iotex-analytics/

RUN apt-get install -y --no-install-recommends make

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN rm -rf ./bin/server && \
    CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o ./bin/server -v .

FROM alpine:3.11

RUN mkdir -p /etc/iotex/
COPY --from=build /go/apps/iotex-analytics/bin/server /usr/local/bin/iotex-server
COPY --from=build /go/apps/iotex-analytics/config.yaml /etc/iotex/config.yaml

CMD [ "iotex-server"]
