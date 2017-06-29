package tcpConfig

import (
	"strings"
	"encoding/json"
	"time"
	"net"

	log "github.com/Sirupsen/logrus"

	. "github.com/KharkivGophers/center-smart-house/common"
	. "github.com/KharkivGophers/center-smart-house/common/models"
	"github.com/KharkivGophers/center-smart-house/dao"
)

type TCPConfigServer struct {
	DBURL

	Host string
	Port string

	reconnect *time.Ticker
	pool      ConnectionPool
	Messages  chan []string
}

func NewTCPConfigServer(host, port string, dburl DBURL, reconnect *time.Ticker, messages  chan []string) *TCPConfigServer {
	return &TCPConfigServer{
		DBURL:     dburl,
		Host:      host,
		Port:      port,
		reconnect: reconnect,
		Messages:  messages,
	}
}

func (server *TCPConfigServer) RunConfigServer() {

	server.pool.Init()

	myRedis, err := dao.MyRedis{Host: server.DbHost, Port: server.DbPort}.RunDBConnection()
	defer myRedis.Close()
	CheckError("TCP Connection: runConfigServer", err)

	ln, err := net.Listen("tcp", server.Host+":"+server.Port)

	for err != nil {

		for range server.reconnect.C {
			ln, _ = net.Listen("tcp", server.Host+":"+server.Port)
		}
		server.reconnect.Stop()
	}
	go server.configSubscribe("configChan", server.Messages, &server.pool)

	for {
		conn, err := ln.Accept()
		CheckError("TCP config conn Accept", err)
		go server.sendDefaultConfiguration(conn, &server.pool)
	}
}

func (server *TCPConfigServer) sendNewConfiguration(config DevConfig, pool *ConnectionPool) {

	connection := pool.GetConn(config.MAC)
	if connection == nil {
		log.Error("Has not connection with mac:config.MAC  in connectionPool")
		return
	}

	// log.Println("mac in pool sendNewConfig", config.MAC)
	err := json.NewEncoder(connection).Encode(&config)

	if err != nil {
		pool.RemoveConn(config.MAC)
	}
	CheckError("sendNewConfig", err)
}

func (server *TCPConfigServer) sendDefaultConfiguration(conn net.Conn, pool *ConnectionPool) {
	// Send Default Configuration to Device
	var (
		req    Request
		config *DevConfig
	)

	myRedis, err := dao.MyRedis{Host: server.DbHost, Port: server.DbPort}.RunDBConnection()
	defer myRedis.Close()
	CheckError("DBConnection Error in ----> sendDefaultConfiguration", err)
	err = json.NewDecoder(conn).Decode(&req)
	CheckError("sendDefaultConfiguration JSON Decod", err)

	pool.AddConn(conn, req.Meta.MAC)

	configInfo := req.Meta.MAC + ":" + "config" // key

	if ok, _ := myRedis.Client.Exists(configInfo); ok {

		state, err := myRedis.Client.HMGet(configInfo, "TurnedOn")
		CheckError("Get from DB error1: TurnedOn ", err)

		if strings.Join(state, " ") != "" {
			config = myRedis.GetFridgeConfig(configInfo, req.Meta.MAC)
			log.Println("Old Device with MAC: ", req.Meta.MAC, "detected.")
		}

	} else {
		log.Warningln("New Device with MAC: ", req.Meta.MAC, "detected.")
		log.Warningln("Default Config will be sent.")
		config = CreateDefaultConfigToFridge()
		myRedis.SetFridgeConfig(configInfo, config)
	}

	err = json.NewEncoder(conn).Encode(&config)
	CheckError("sendDefaultConfiguration JSON enc", err)
	log.Warningln("Configuration has been successfully sent")
}

func (server *TCPConfigServer) configSubscribe(roomID string, message chan []string, pool *ConnectionPool) {
	myRedis, err := dao.MyRedis{Host: server.DbHost, Port: server.DbPort}.RunDBConnection()
	defer myRedis.Close()
	CheckError("TCP Connection: runConfigServer", err)

	myRedis.Subscribe(message, roomID)
	for {
		var config DevConfig
		select {
		case msg := <-message:
			if msg[0] == "message" {
				err := json.Unmarshal([]byte(msg[2]), &config)
				CheckError("configSubscribe: unmarshal", err)
				go server.sendNewConfiguration(config, pool)
			}
		}
	}
}

// Only fridge. must be refactored------------------------------------------------------------

func CreateDefaultConfigToFridge() *DevConfig {
	return &DevConfig{
		TurnedOn:    true,
		StreamOn:    true,
		CollectFreq: 1000,
		SendFreq:    5000,
	}
}
