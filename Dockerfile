FROM golang:1.11.5-stretch

WORKDIR $GOPATH/src/github.com/iotexproject/iotex-analytics/

RUN apt-get install -y --no-install-recommends make

COPY . .

ARG SKIP_DEP=false

RUN if [ "$SKIP_DEP" != true ] ; \
    then \
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && \
        dep ensure --vendor-only; \
    fi

RUN rm -rf ./bin/server && \
    rm -rf analytics.db && \
    go build -o ./bin/server -v ./graphql/server && \
    cp $GOPATH/src/github.com/iotexproject/iotex-analytics/bin/server /usr/local/bin/iotex-server  && \
    mkdir -p /etc/iotex/ && \
    cp config.yaml /etc/iotex/config.yaml && \
    rm -rf $GOPATH/src/github.com/iotexproject/iotex-analytics/

CMD [ "iotex-server", "-config=/etc/iotex/config.yaml" "-chain-endpoint=130.211.201.187:80", "-election-endpoint=35.232.228.38:8089"]