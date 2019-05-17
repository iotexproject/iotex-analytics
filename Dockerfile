FROM golang:1.12.5-stretch

WORKDIR apps/iotex-analytics/

RUN apt-get install -y --no-install-recommends make

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN rm -rf ./bin/server && \
    rm -rf analytics.db && \
    go build -o ./bin/server -v . && \
    cp ./bin/server /usr/local/bin/iotex-server  && \
    mkdir -p /etc/iotex/ && \
    cp config.yaml /etc/iotex/config.yaml && \
    rm -rf apps/iotex-analytics/

CMD [ "iotex-server", "-config=/etc/iotex/config.yaml", "-chain-endpoint=130.211.201.187:80", "-election-endpoint=35.232.228.38:8089"]