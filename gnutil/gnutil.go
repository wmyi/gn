package gnutil

import (
	"hash/crc32"

	"github.com/gogo/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
	"github.com/wmyi/gn/config"
	logger "github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
)

var (
	jsonI = jsoniter.ConfigCompatibleWithStandardLibrary
)

func RPCcalculatorServerId(calculateId string, serverList []*config.ServersConfig) (string, error) {
	index := int(crc32.ChecksumIEEE([]byte(calculateId))) % len(serverList)

	if serverList[index] != nil {
		return serverList[index].ID, nil
	}
	return "", gnError.ErrNotCalculateServerId
}

func JsonToBytes(obj interface{}) ([]byte, bool) {
	if obj != nil {
		out, err := jsonI.Marshal(obj)
		if err != nil {
			logger.Errorf("Pack  ResultJson  jsonI.Marshal  err  ", err)
			return nil, false
		}
		return out, true
	}
	return nil, false
}

func ProtoBufToBytes(obj interface{}) ([]byte, bool) {
	if obj != nil {
		pbObj, ok := obj.(proto.Message)
		if !ok {
			logger.Errorf("Pack  ResultProtoBuf  obj is no proto.Message  type    ")
			return nil, false
		}
		out, err := proto.Marshal(pbObj)
		if err != nil {
			logger.Errorf("Pack  ResultJson  proto.Buffer.Marshal  err     ", err)
			return nil, false
		}
		return out, true
	}
	return nil, false
}
