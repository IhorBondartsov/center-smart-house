package driver

import (
	. "github.com/KharkivGophers/center-smart-house/models"
	. "github.com/KharkivGophers/center-smart-house/dao"

)

type ConfigDevDriver interface {
	GetDevConfig(configInfo, mac string, worker RedisClient) (*DevConfig)
	SetDevConfig(configInfo string, config *DevConfig, worker RedisClient)
	ValidateDevData(config DevConfig) (bool, string)
}

type DataDevDriver interface {
	GetDevData(devParamsKey string, devParamsKeysTokens []string, worker RedisClient) DevData
	SetDevData(req *Request, worker RedisClient) *ServerError
}
