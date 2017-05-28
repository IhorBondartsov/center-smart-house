package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"menteslibres.net/gosexy/redis"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	connChanal  = make(chan *websocket.Conn)
	stopCloseWS = make(chan string)
	stopSub     = make(chan bool)
	mapConn     = make(map[string]*listConnection)

	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,

		//CheckOrigin I think it is bad practice
		CheckOrigin: func(r *http.Request) bool {
			if r.Host == connHost+":"+wsConnPort {
				return true
			}
			return false
		},
	}
)

func tcpDataHandler(conn *net.Conn) {
	var req Request
	var res Response
	for {
		err := json.NewDecoder(*conn).Decode(&req)
		if err != nil {
			log.Errorln("requestHandler JSON Decod", err)
			return
		}
		//sends resp struct from  devTypeHandler by channel;
		go devTypeHandler(req)

		log.Println("Data has been received")

		res = Response{
			Status: http.StatusOK,
			Descr:  "Data have been delivered successfully",
		}
		err = json.NewEncoder(*conn).Encode(&res)
		CheckError("tcpDataHandler JSON enc", err)
	}

}

func devTypeHandler(req Request) {
	switch req.Action {
	case "update":
		switch req.Meta.Type {
		case "fridge":
			if err := req.fridgeDataHandler(); err != nil {
				log.Errorf("%v", err.Error)
			}
		case "washer":
			if err := req.washerDataHandler(); err != nil {
				log.Errorf("%v", err.Error)
			}

		default:
			log.Println("Device request: unknown device type")
			return
		}
		go publishMessage(req, roomIDForDevWSPublish)

	default:
		log.Println("Device request: unknown action")
	}
}

func (req *Request) fridgeDataHandler() *ServerError {
	var devData FridgeData
	mac := req.Meta.MAC
	devReqTime := req.Time
	devType := req.Meta.Type
	devName := req.Meta.Name

	devKey := "device" + ":" + devType + ":" + devName + ":" + mac
	devParamsKey := devKey + ":" + "params"

	_, err := dbClient.SAdd("devParamsKeys", devParamsKey)
	CheckError("DB error", err)
	_, err = dbClient.HMSet(devKey, "ReqTime", devReqTime)
	CheckError("DB error", err)
	_, err = dbClient.SAdd(devParamsKey, "TempCam1", "TempCam2")
	CheckError("DB error", err)

	err = json.Unmarshal([]byte(req.Data), &devData)
	if err != nil {
		return &ServerError{Error: err}
	}

	for time, value := range devData.TempCam1 {
		_, err := dbClient.ZAdd(devParamsKey+":"+"TempCam1",
			int64ToString(time), int64ToString(time)+":"+float32ToString(float64(value)))
		CheckError("DB error", err)
		return &ServerError{Error: err}
	}

	for time, value := range devData.TempCam2 {
		_, err := dbClient.ZAdd(devParamsKey+":"+"TempCam2",
			int64ToString(time), int64ToString(time)+":"+float32ToString(float64(value)))
		CheckError("DB error", err)
		return &ServerError{Error: err}
	}

	return nil
}

func (req *Request) washerDataHandler() *ServerError {
	//to be continued
	return nil
}

func float32ToString(num float64) string {
	return strconv.FormatFloat(num, 'f', -1, 32)
}

func int64ToString(n int64) string {
	return strconv.FormatInt(int64(n), 10)
}

func getDevicesHandler(w http.ResponseWriter, r *http.Request) {
	var device DeviceView
	var devices []DeviceView
	devParamsKeys, _ := dbClient.SMembers("devParamsKeys")

	var devParamsKeysTokens = make([][]string, len(devParamsKeys))
	for i, k := range devParamsKeys {
		devParamsKeysTokens[i] = strings.Split(k, ":")
	}

	for index, key := range devParamsKeysTokens {
		params, _ := dbClient.SMembers(devParamsKeys[index])

		device.Meta.Type = key[1]
		device.Meta.Name = key[2]
		device.Meta.MAC = key[3]
		device.Data = make(map[string][]string)

		values := make([][]string, len(params))
		for i, p := range params {
			values[i], _ = dbClient.ZRangeByScore(devParamsKeys[index]+":"+p, "-inf", "inf")
			device.Data[p] = values[i]
		}

		devices = append(devices, device)
	}

	err := json.NewEncoder(w).Encode(devices)
	CheckError("getDevicesHandler JSON enc", err)
}

func getDevDataHandler(w http.ResponseWriter, r *http.Request) {
	var device DetailedDevData
	vars := mux.Vars(r)
	devID := "device:" + vars["id"]

	devParamsKeysTokens := []string{}
	devParamsKeysTokens = strings.Split(devID, ":")
	devParamsKey := devID + ":" + "params"

	params, _ := dbClient.SMembers(devParamsKey)
	device.Meta.Type = devParamsKeysTokens[1]
	device.Meta.Name = devParamsKeysTokens[2]
	device.Meta.MAC = devParamsKeysTokens[3]
	device.Data = make(map[string][]string)

	values := make([][]string, len(params))
	for i, p := range params {
		values[i], _ = dbClient.ZRangeByScore(devParamsKey+":"+p, "-inf", "inf")
		device.Data[p] = values[i]
	}

	err := json.NewEncoder(w).Encode(device)
	CheckError("getDevDataHandler JSON enc", err)
}

func postDevConfigHandler(w http.ResponseWriter, r *http.Request) {
	var config Config
	vars := mux.Vars(r)

	id := "device:" + vars["id"]
	mac := vars["id"]
	configInfo := vars["if"] + ":" + "data"

	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	Time := time.Now().UnixNano() / int64(time.Millisecond)
	log.Println("Received configuration: ", config, "id: ", id)
	w.WriteHeader(http.StatusOK)

	// Validate MAC
	validateMAC(mac, w)
	// Validate State
	validateState(config, w)
	// Validate Collect Frequency
	validateCollectFreq(config, w)
	// Validate Send Frequency
	validateSendFreq(config, w)

	// Save to DB
	_, err = dbClient.SAdd("Config", configInfo)
	CheckError("DB error", err)
	_, err = dbClient.HMSet(vars["id"], "ConfigTime", Time)
	CheckError("DB error", err)
	_, err = dbClient.SAdd(configInfo, "TurnedOn", "CollectFreq", "SendFreq")
	CheckError("DB error", err)
	_, err = dbClient.ZAdd(configInfo+":"+"TurnedOn", config.TurnedOn)
	CheckError("DB error", err)
	_, err = dbClient.ZAdd(configInfo+":"+"CollectFreq", config.CollectFreq)
	CheckError("DB error", err)
	_, err = dbClient.ZAdd(configInfo+":"+"SendFreq", config.SendFreq)
	CheckError("DB error", err)

	go publishMessage(r.Body, "configChan")

	log.Println("Received configuration: ", config, "id: ", id)
	w.WriteHeader(http.StatusOK)
}

func validateSendFreq(c Config, w http.ResponseWriter) {
	switch reflect.TypeOf(c.SendFreq).String() {
	case "int":
		switch {
		case c.SendFreq > 0 && c.SendFreq < 100:
			return
		default:
			http.Error(w, "0 < Send Frequency < 100", 400)
			break
		}
	default:
		http.Error(w, "Send Frequency should be in integer format", 415)
		break
	}
}

func validateCollectFreq(c Config, w http.ResponseWriter) {
	switch reflect.TypeOf(c.CollectFreq).String() {
	case "int":
		switch {
		case c.CollectFreq > 0 && c.CollectFreq < 100:
			return
		default:
			http.Error(w, "0 < Collect Frequency < 100", 400)
			break
		}
	default:
		http.Error(w, "Collect Frequency should be in integer format", 415)
		break
	}
}

func validateState(c Config, w http.ResponseWriter) {
	switch reflect.TypeOf(c.TurnedOn).String() {
	case "bool":
		return
	default:
		http.Error(w, "State should be in byte format", 415)
		break
	}
}

func validateMAC(mac string, w http.ResponseWriter) {
	switch reflect.TypeOf(mac).String() {
	case "string":
		fmt.Println("This is string")
		switch len(mac) {
		case 17:
			return
		default:
			http.Error(w, "MAC should contain 17 symbols", 400)
			break
		}
	default:
		http.Error(w, "MAC should be in string format", 415)
		break
	}
}

//-------------------WEB Socket Handler -----------------------
func webSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}
	//http://..../device/type/name/mac
	uri := strings.Split(r.URL.String(), "/")

	if _, ok := mapConn[uri[4]]; !ok {
		mapConn[uri[4]] = new(listConnection)
	}
	mapConn[uri[4]].Add(conn)
}

/**
using with tcp.
*/
func publishMessage(message interface{}, roomID string) {
	_, err := dbClient.Publish(roomID, message)
	CheckError("Publish", err)
}

/**
Delete connections from mapConn
*/
func CloseWebsocket() {
	for {
		select {
		case connAddres := <-connChanal:
			for _, val := range mapConn {
				if ok := val.Remove(connAddres); ok {
					break
				}
			}
		case <-stopCloseWS:
			log.Info("CloseWebsocket closed")
			return
		}

	}
}

func subscribe(client *redis.Client, roomID string, channel chan []string) {
	client = redis.New()
	err := client.ConnectNonBlock(dbHost, dbPort)
	CheckError("Subscribe", err)

	go client.Subscribe(channel, roomID)
}

func configSubscribe(client *redis.Client, roomID string, messages chan []string, pool *ConectionPool) {
	subscribe(client, roomID, messages)
	var cnfg Config

	for msg := range messages {

		for _, v := range msg {
			log.Warningln(v)
		}

		err := json.Unmarshal([]byte(msg[2]), &cnfg)
		if CheckError("configSubscribe", err) != nil {
			return
		}
		sendNewConfiguration(cnfg, pool)
	}
}

func WSSubscribe(client *redis.Client, roomID string, channel chan []string) {
	subscribe(client, roomID, channel)
	for {
		select {
		case msg := <-channel:
			go checkAndSendInfoToWSClient(msg)
		case <-stopSub:
			log.Info("Subscribe closed")
			return
		}
	}
}

//We are check mac in our mapConnections.
// If we have mac in the map we will send message to all connections.
// Else we do nothing
func checkAndSendInfoToWSClient(msg []string) {
	fmt.Println(msg)
	r := new(Request)
	err := json.Unmarshal([]byte(msg[2]), r)
	CheckError("checkAndSendInfoToWSClient", err)
	fmt.Println(r)
	if _, ok := mapConn[r.Meta.MAC]; ok {
		sendInfoToWSClient(r.Meta.MAC, msg[2])
	}
}

//Send message to all connections which we have in map, and which pertain to mac
func sendInfoToWSClient(mac, message string) {
	mapConn[mac].Lock()
	for _, val := range mapConn[mac].connections {
		fmt.Println(message)
		err := val.WriteJSON(message)
		if err != nil {
			log.Error("connection closed")
			go getToChanal(val)
		}
	}
	mapConn[mac].Unlock()
}

func getToChanal(conn *websocket.Conn) {
	connChanal <- conn
}

func CheckError(desc string, err error) error {
	if err != nil {
		log.Errorln(desc, err)
		return err
	}
	return nil
}
