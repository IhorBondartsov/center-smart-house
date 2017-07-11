package dao
import (
	. "github.com/KharkivGophers/center-smart-house/models"

)

// Abstract database interface.
type DbClient interface {
	FlushAll() (error)
	Publish(channel string, message interface{}) (int64, error)
	Connect()(error)
	Subscribe(cn chan []string, channel ...string) error
	Close() (error)
	NewDBConnection()(DbClient)
	GetAllDevices() ([]DevData)
	GetClient() DbRedisDriver

	GetKeyForConfig(mac string)string
	SetDBServer(server Server)

}

// Concrete redis database interface.
type DbRedisDriver interface {
	SAdd(key string, member ...interface{}) (int64, error)
	ZAdd(key string, arguments ...interface{}) (int64, error)
	ZRem(key string, arguments ...interface{}) (int64, error)
	ZRange(key string, values ...interface{}) ([]string, error)
	ZRangeByScore(key string, values ...interface{}) ([]string, error)
	ZScore(key string, member interface{}) (int64, error)

	HMSet(key string, values ...interface{}) (string, error)
	HMGet(key string, fields ...string) ([]string, error)

	SMembers(key string) ([]string, error)

	Close() error
	Connect(host string, port uint) (err error)

	Exists(key string) (bool, error)

	Multi() (string, error)
	Discard()  (string, error)
	Exec()  ([]interface{}, error)


}
