FROM golang:1.11.5-stretch

WORKDIR $GOPATH/src/github.com/iotexproject/iotex-analytics/

RUN apt-get install -y --no-install-recommends make

COPY . .

RUN rm -rf ./bin/server && \
    rm -rf analytics.db && \
    go build -o ./bin/server -v ./graphql/server && \
    cp $GOPATH/src/github.com/iotexproject/iotex-analytics/bin/server /usr/local/bin/iotex-server  && \
    mkdir -p /etc/iotex/ && \
    cp config.yaml /etc/iotex/config.yaml && \
    rm -rf $GOPATH/src/github.com/iotexproject/iotex-analytics/

CMD [ "iotex-server", "-config=/etc/iotex/config.yaml"]