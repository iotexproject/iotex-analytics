# iotex-analytics
The independent service that analyzes data from IoTeX blockchain

## Get started

### Minimum requirements

| Components | Version | Description |
|----------|-------------|-------------|
| [Golang](https://golang.org) | &ge; 1.11.5 | Go programming language |

## Run as a service
1. If you put the project code under your `$GOPATH/src`, you will need to set up an environment variable:
```
export GO111MODULE=on
```

2. Specify MySQL connection string and datababse name by setting up the following environment variables:
```
export CONNECTION_STRING=username:password@protocol(address)/
export DB_NAME=dbname
```
e.g. 
```
export CONNECTION_STRING=root:rootuser@tcp(127.0.0.1:3306)/
export DB_NAME=analytics
```
Note that you need to set up a MySQL DB instance beforehand.

3. Specify IoTeX Public API address and IoTeX election service address by setting up the following environment variables:
```
export CHAIN_ENDPOINT=Full_Node_IP:API_Port
export ELECTION_ENDPOINT=Election_Host_IP:Election_Port
```
If you don't have access to an IoTeX full node, you can use the following setups:
```
export CHAIN_ENDPOINT=35.233.188.105:14014
export ELECTION_ENDPOINT=35.233.188.105:8089
```

4. Specify server port (OPTIONAL):
```
export PORT=Port_Number
```
Port number = 8089 by default

5. Start IoTeX-Analytics server:
```
make run
```

6. If you want to query analytical data through GraphQL playground, after starting the server, go to http://localhost:8089/

You need to change the port number if you specify a different one. 

## Start a service in Docker Container

You can find the docker image on [docker hub](https://hub.docker.com/r/iotex/iotex-analytics).

1. Pull the docker image:

```
docker pull iotex/iotex-analytics:v0.1.0
```

2. Run the following command to start a node:

```
docker run -d --restart on-failure --name analytics \
        -p 8089:8089 \
        -e CONFIG=/etc/iotex/config.yaml \
        -e CHAIN_ENDPOINT=35.233.188.105:14014 \
        -e ELECTION_ENDPOINT=35.233.188.105:8089 \
        -e CONNECTION_STRING=root:rootuser@tcp(host.docker.internal:3306)/ \
        -e DB_NAME=analytics \
        iotex/iotex-analytics:v0.1.0 \
        iotex-server
```

Note that you might need to change environment variables above based on your settings. 

Now the service should be started successfully.


