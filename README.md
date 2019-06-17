# iotex-analytics
The independent service that analyzes data from IoTeX blockchain

## Run as a service
1. If you put the project code under your `$GOPATH\src`, you will need to set up an environment variable
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
