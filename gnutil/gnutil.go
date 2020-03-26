package gnutil

import (
	"hash/crc32"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/gnError"
)

func RPCcalculatorServerId(calculateId string, serverList []*config.ServersConfig) (string, error) {
	index := int(crc32.ChecksumIEEE([]byte(calculateId))) % len(serverList)

	if serverList[index] != nil {
		return serverList[index].ID, nil
	}
	return "", gnError.ErrNotCalculateServerId
}
