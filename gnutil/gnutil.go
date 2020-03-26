package gnutil

import (
	"gn/config"
	"gn/gnError"
	"hash/crc32"
)

func RPCcalculatorServerId(calculateId string, serverList []*config.ServersConfig) (string, error) {
	index := int(crc32.ChecksumIEEE([]byte(calculateId))) % len(serverList)

	if serverList[index] != nil {
		return serverList[index].ID, nil
	}
	return "", gnError.ErrNotCalculateServerId
}
