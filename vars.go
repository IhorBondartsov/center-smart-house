package main

import (
	"net/http"
	"sync"
	"github.com/gorilla/websocket"
	"os"
	"strconv"
)

//var from main.go
var (
	//Database
	dbHost     = getEnvDbHost("REDIS_PORT_6379_TCP_ADDR")
	dbPort     = getEnvDbPort("REDIS_PORT_6379_TCP_PORT")
	//General
	connHost = "0.0.0.0"

	//tcp conn with devices
	connType    = "tcp"
	tcpConnPort = "3030"

	//http connection
	httpConnPort = "8100"

	//for TCP config
	configConnType = "tcp"
	configHost     = "0.0.0.0"
	configPort     = "3000"

	//Web-socket connections
	wsConnPort            = "2540"
	roomIDForDevWSPublish = "devWS"


	wg    sync.WaitGroup

	//This var is using by websocket
	mapConn     = make(map[string]*listConnection)
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			//if r.Host == connHost+":"+wsConnPort {
			//	return true
			//}
			return true
		},
	}
)

func getEnvDbPort(key string) uint {
	parsed64, _ := strconv.ParseUint(os.Getenv(key), 10, 64)
	port := uint(parsed64)
	if port == 0 {
		return uint(6379)
	}
	return port
}

func getEnvDbHost(key string) string {
	host := os.Getenv(key)
	if len(host) == 0 {
		return "127.0.0.1"
	}
	return host
}

