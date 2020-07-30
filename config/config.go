package config

import (
	"errors"
	"io/ioutil"
	"os"

	jsoniter "github.com/json-iterator/go"
)

var (
	jsonI = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	CMD_PING string = "ping"
	CMD_MEM  string = "mem"
)

type ServersConfig struct {
	ID               string `json:"id"`
	ServerType       string `json:"serverType"`
	HandleTimeOut    int    `json:"handleTimeOut"`
	RPCTimeOut       int    `json:"rpcTimeOut"`
	Host             string `json:"host"` // rpc host使用，nats RPC的时候不需要这两个值
	Port             string `json:"port"` // rpc 端口   nats 不需要
	MaxRunRoutineNum int    `json:"maxRunRoutineNum"`
}

type ConnectorConfig struct {
	ID         string `json:"id"`
	Host       string `json:"host"`
	ClientPort int    `json:"clientPort"`
	EndPort    int    `json:"endPort"`
	Frontend   bool   `json:"frontend"`
	HeartTime  int    `json:"heartTime"`
	ServerType string `json:"serverType"`
}
type Natsconfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type LogConfig struct {
	Encoding   string `json:"encoding"`
	Level      string `json:"level"`
	MaxLogSize int64  `json:"maxLogSize"`
	NumBackups int    `json:"numBackups"`
}

type MasterConfig struct {
	ID             string `json:"id"`
	NodeHeartBeart int    `json:"nodeHeartBeart"`
}

type Config struct {
	Natsconf   Natsconfig         `json:"natsconfig"`
	MasterConf MasterConfig       `json:"master"`
	LogConf    LogConfig          `json:"log"`
	Rpctype    string             `json:"rpctype"` // rpc type  nats  or  rpc
	Connector  []*ConnectorConfig `json:"connector"`
	Servers    []*ServersConfig   `json:"servers"`

	typeServer map[string][]*ServersConfig
}

func (c *Config) init() error {
	if len(c.Servers) > 0 {
		c.typeServer = make(map[string][]*ServersConfig)
		for index := 0; index < len(c.Servers); index++ {
			if c.typeServer[c.Servers[index].ServerType] == nil {
				c.typeServer[c.Servers[index].ServerType] = make([]*ServersConfig, 0, 10)
			}
			c.typeServer[c.Servers[index].ServerType] =
				append(c.typeServer[c.Servers[index].ServerType], c.Servers[index])
		}
		return nil
	}
	return errors.New("config  init  eror  config ")
}

func (c *Config) GetMasterConf() *MasterConfig {
	return &c.MasterConf
}

func (c *Config) GetConConfByServerId(serverId string) *ConnectorConfig {
	if len(serverId) > 0 && len(c.Connector) > 0 {
		for _, value := range c.Connector {
			if value.ID == serverId {
				return value
			}
		}
	}
	return nil
}

func (c *Config) GetServerConfByServerId(serverId string) *ServersConfig {
	if len(serverId) > 0 && len(c.Servers) > 0 {
		for _, value := range c.Servers {
			if value.ID == serverId {
				return value
			}
		}
	}

	return nil
}

func (c *Config) GetServerByType(serverType string) []*ServersConfig {
	if c.typeServer == nil || c.Servers == nil {
		return nil
	}

	return c.typeServer[serverType]
}

func NewConfig(filePath string) (*Config, error) {
	var config Config

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	err = jsonI.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}
	err = config.init()
	if err != nil {
		return nil, err
	} else {
		return &config, nil
	}
}
