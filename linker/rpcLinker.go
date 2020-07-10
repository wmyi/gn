package linker

import (
	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"go.etcd.io/etcd/client"
)

type RpcLinker struct {
	CWChan        chan []byte
	serverAddr    string
	serverID      string
	isRuning      bool
	exDetect      *gnError.GnExceptionDetect
	conf          *config.Config
	clientConMaps map[string]*client.Client
}

func NewRpcxLinker(serverId string, config *config.Config, address string, outChan chan []byte, log *glog.Glogger,
	detect *gnError.GnExceptionDetect) ILinker {
	rpcx := &RpcLinker{
		CWChan:        outChan,
		serverAddr:    address,
		serverID:      serverId,
		isRuning:      false,
		exDetect:      detect,
		clientConMaps: make(map[string]*client.Client, 1<<4), // grpc clients  serverId:rpcxClient
		conf:          config,
	}
	return rpcx
}

func (rl *RpcLinker) SendMsg(router string, data []byte) {
	if len(router) > 0 && len(data) > 0 {
		if con, ok := rl.clientConMaps[router]; ok && con != nil {
		} else {

		}
	}
}
func (rl *RpcLinker) GetSubRounter() string {
	return rl.serverID
}
func (rl *RpcLinker) Run() error {
	if !rl.isRuning {

		rl.isRuning = true
	}
	return nil
}
func (rl *RpcLinker) GetConnection() interface{} {
	return nil
}
func (rl *RpcLinker) Done() {

}
