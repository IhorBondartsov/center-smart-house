package devices

import (
	"net"
	"time"
	"net/http"
	"encoding/json"

	log "github.com/Sirupsen/logrus"

	. "github.com/KharkivGophers/center-smart-house/sys"
	. "github.com/KharkivGophers/center-smart-house/dao"
	. "github.com/KharkivGophers/center-smart-house/models"
)

/**
Time keep
 */
type Washer struct {
	Data   WasherData
	Config WasherConfig
	Meta   DevMeta
	timeStartWash int64
}

type WasherData struct {
	Turnovers map[int64]int64        `json:"turnovers"`
	WaterTemp map[int64]float32      `json:"waterTemp"`
}

type WasherConfig struct {
	Name           string   `json:"name"`
	MAC            string   `json:"mac"`
	Temperature    float32  `json:"temperature"`
	WashTime       int64    `json:"washTime"`
	WashTurnovers  int64    `json:"washTurnovers"`
	RinseTime      int64    `json:"rinseTime"`
	RinseTurnovers int64    `json:"rinseTurnovers"`
	SpinTime       int64    `json:"spinTime"`
	SpinTurnovers  int64    `json:"spinTurnovers"`
}
type TimerMode struct {
	Name      string   `json:"name"`
	StartTime int64    `json:time`
}

var (
	LightMode WasherConfig = WasherConfig{
		Name:           "LightMode",
		Temperature:    60,
		WashTime:       90,
		WashTurnovers:  240,
		RinseTime:      30,
		RinseTurnovers: 120,
		SpinTime:       30,
		SpinTurnovers:  60,
	}
	FastMode WasherConfig = WasherConfig{
		Name:           "FastMode",
		Temperature:    180,
		WashTime:       30,
		WashTurnovers:  300,
		RinseTime:      15,
		RinseTurnovers: 240,
		SpinTime:       15,
		SpinTurnovers:  60,
	}
	StandartMode WasherConfig = WasherConfig{
		Name:           "StandartMode",
		Temperature:    240,
		WashTime:       120,
		WashTurnovers:  240,
		RinseTime:      60,
		RinseTurnovers: 180,
		SpinTime:       60,
		SpinTurnovers:  60,
	}
)

func (washer *Washer) selectMode(mode string) (WasherConfig) {
	switch mode {
	case "LightMode":
		washer.Config = LightMode
		return LightMode
	case "FastMode":
		washer.Config = FastMode
		return FastMode
	case "StandartMode":
		washer.Config = StandartMode
		return StandartMode
	}
	return WasherConfig{}
}


//--------------------------------------DevDataDriver--------------------------------------------------------------
func (washer *Washer) GetDevData(devParamsKey string, devMeta DevMeta, client DbClient) DevData {
	var device DevData

	params, err := client.GetClient().SMembers(devParamsKey)
	CheckError("Cant read members from devParamsKeys", err)
	device.Meta = devMeta
	device.Data = make(map[string][]string)

	values := make([][]string, len(params))
	for i, p := range params {
		values[i], err = client.GetClient().ZRangeByScore(devParamsKey+":"+p, "-inf", "inf")
		CheckError("Cant use ZRangeByScore", err)
		device.Data[p] = values[i]
	}
	return device
}

func (washer *Washer) SetDevData(req *Request, client DbClient) *ServerError {

	var devData WasherData

	devKey := "device" + ":" + req.Meta.Type + ":" + req.Meta.Name + ":" + req.Meta.MAC
	devParamsKey := devKey + ":" + "params"

	err := json.Unmarshal([]byte(req.Data), &devData)
	if err != nil {
		log.Errorf("Error in SetDevData. %v", err)
		return &ServerError{Error: err}
	}

	client.GetClient().Multi()
	err = setTurnoversData(devData.Turnovers, devParamsKey+":"+"Turnovers", client)
	err = setWaterTempData(devData.WaterTemp, devParamsKey+":"+"WaterTemp", client)
	_, err = client.GetClient().Exec()

	if CheckError("Error in SetDevData", err) != nil {
		client.GetClient().Discard()
		return &ServerError{Error: err}
	}

	return nil
}

func setWaterTempData(TempCam map[int64]float32, key string, client DbClient) error {
	for time, value := range TempCam {
		client.GetClient().ZAdd(key, Int64ToString(time), Int64ToString(time)+":"+Float64ToString(value))

	}
	return nil
}
func setTurnoversData(TempCam map[int64]int64, key string, client DbClient) error {
	for time, value := range TempCam {
		client.GetClient().ZAdd(key, Int64ToString(time), Int64ToString(time)+":"+Int64ToString(value))
	}
	return nil
}

//--------------------------------------DevConfigDriver--------------------------------------------------------------

func (washer *Washer) GetDevConfig(configInfo, mac string, client DbClient) (*DevConfig) {
	return &DevConfig{}
}

func (washer *Washer) SetDevConfig(configInfo string, config *DevConfig, client DbClient) {
	var timerMode *TimerMode
	json.Unmarshal(config.Data, timerMode)
	client.GetClient().ZAdd(configInfo, timerMode.StartTime, timerMode.Name)
}

func (washer *Washer) GetDefaultConfig() (*DevConfig) {
	b, _ := json.Marshal(WasherConfig{})
	return &DevConfig{Data: b}
}


//--------------------------------------DevServerHandler--------------------------------------------------------------

func (washer *Washer) SendDefaultConfigurationTCP(conn net.Conn, dbClient DbClient, req *Request) ([]byte) {
	var config *DevConfig
	configInfo := req.Meta.MAC + ":" + "config" // key

	if ok, _ := dbClient.GetClient().Exists(configInfo); ok {
		t := time.Now().UnixNano() / int64(time.Minute)
		config = washer.getActualConfig(configInfo, req.Meta.MAC, dbClient,t)
		log.Println("Old Device with MAC: ", req.Meta.MAC, "detected.")

	} else {
		log.Warningln("New Device with MAC: ", req.Meta.MAC, "detected.")
		log.Warningln("Default Config will be sent.")
		config = washer.GetDefaultConfig()
		washer.saveDeviceToBD(configInfo, config, dbClient, req)
	}
	return config.Data
}

func (washer *Washer) PatchDevConfigHandlerHTTP(w http.ResponseWriter, r *http.Request, meta DevMeta, client DbClient) {

}

func (washer *Washer) getActualConfig(configInfo, mac string, client DbClient, unixTime int64) (*DevConfig) {
	config := washer.GetDefaultConfig()
	config.MAC = mac

	mode, err := client.GetClient().ZRangeByScore(configInfo, unixTime-100, unixTime+100)
	if err != nil {
		CheckError("Washer. GetDevConfig. Cant perform ZRangeByScore", err)
	}
	log.Info("Washer. GetDevConfig. Mode ", mode, "Time ", unixTime)

	if len(mode) == 0 {
		return config
	}

	configWasher := washer.selectMode(mode[0])
	config.Data, err = json.Marshal(configWasher)
	CheckError("Washer. GetDevConfig. Cant perform json.Marshal(configWasher)", err)
	return config
}


func (washer *Washer) saveDeviceToBD(configInfo string, config *DevConfig, client DbClient, req *Request) {
	var timerMode TimerMode
	json.Unmarshal(config.Data, &timerMode)

	devKey := "device" + ":" + req.Meta.Type + ":" + req.Meta.Name + ":" + req.Meta.MAC
	devParamsKey := devKey + ":" + "params"

	client.GetClient().Multi()
	client.GetClient().SAdd("devParamsKeys", devParamsKey)
	client.GetClient().HMSet(devKey, "ReqTime", req.Time)
	client.GetClient().SAdd(devParamsKey, "Turnovers", "WaterTemp")
	client.GetClient().ZAdd(configInfo, timerMode.StartTime, timerMode.Name)
	_, err := client.GetClient().Exec()

	if CheckError("DB error14", err) != nil {
		client.GetClient().Discard()
	}
}