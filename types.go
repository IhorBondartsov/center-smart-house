package main

import (
	"encoding/json"
	"net"
	"sync"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

type ServerError struct {
	Error   error
	Message string
	Code    int
}

type Response struct {
	Status int    `json:"status"`
	Descr  string `json:"descr"`
}

type DevConfig struct {
	TurnedOn bool `json:"turnedOn"`
	StreamOn    bool   `json:"streamOn"`
	CollectFreq int64  `json:"collectFreq"`
	SendFreq    int64  `json:"sendFreq"`
	MAC         string `json:"mac"`
}

type DevConfigFreqs struct {
	CollectFreq int64  `json:"collectFreq"`
	SendFreq    int64  `json:"sendFreq"`
	MAC         string `json:"mac"`
}

type DevConfigTurnedOn struct {
	TurnedOn bool   `json:"turnedOn"`
	MAC      string `json:"mac"`
}

type DevMeta struct {
	Type string `json:"type"`
	Name string `json:"name"`
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
}

type Request struct {
	Action string          `json:"action"`
	Time   int64           `json:"time"`
	Meta   DevMeta         `json:"meta"`
	Data   json.RawMessage `json:"data"`
}

type FridgeData struct {
	TempCam1 map[int64]float32 `json:"tempCam1"`
	TempCam2 map[int64]float32 `json:"tempCam2"`
}

type WasherData struct {
	Mode   string
	Drying string
	Temp   map[int64]float32
}

type DevData struct {
	Site string              `json:"site"`
	Meta DevMeta             `json:"meta"`
	Data map[string][]string `json:"data"`
}

//Connections pool for configTCPServer
type ConnectionPool struct {
	sync.Mutex
	conn map[string]*net.Conn
}

func (pool *ConnectionPool) addConn(conn *net.Conn, key string) {
	pool.Lock()
	pool.conn[key] = conn
	defer pool.Unlock()
}

func (pool *ConnectionPool) getConn(key string) *net.Conn {
	pool.Lock()
	defer pool.Unlock()
	return pool.conn[key]
}
func (pool *ConnectionPool) init() {
	pool.Lock()
	defer pool.Unlock()

	pool.conn = make(map[string]*net.Conn)
}

//For work with web socket
type listConnection struct {
	sync.Mutex
	connections []*websocket.Conn
}

func (list *listConnection) Add(conn *websocket.Conn) {
	list.Lock()
	list.connections = append(list.connections, conn)
	list.Unlock()

}
func (list *listConnection) Remove(conn *websocket.Conn) bool {
	list.Lock()
	defer list.Unlock()
	position := 0
	for _, v := range list.connections {
		if v == conn {
			list.connections = append(list.connections[:position], list.connections[position+1:]...)
			log.Info("Web sockets connection deleted: ", conn.RemoteAddr())
			return true
		}
		position++
	}
	return false
}
