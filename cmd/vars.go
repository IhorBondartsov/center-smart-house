package main

import (
	"os"
	"strconv"
	"github.com/KharkivGophers/center-smart-house/models"
)

var (
	dbServer = models.Server{
		IP: getEnvDbHost("REDIS_PORT_6379_TCP_ADDR"),
		Port: getEnvDbPort("REDIS_PORT_6379_TCP_PORT"),
	}

	centerIP = "0.0.0.0"

	// tcp data connection with devices
	dataConnType    = "tcp"
	tcpDevDataPort = uint(3030)

	// http connection with browser
	httpConnPort = "8100"

	// tcp config connection with devices
	configConnType = "tcp"
	tcpDevConfigPort  = uint(3000)

	// web-socket connection
	wsPort            = uint(2540)
	roomIDForDevWSPublish = "devWS"
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

