package linker

import (
	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
)

type RpcxLinker struct {
	CWChan     chan []byte
	serverAddr string
	serverID   string
	isRuning   bool
	exDetect   *gnError.GnExceptionDetect
	conf       *config.Config
}

func NewRpcxLinker(serverId string, config *config.Config, address string, outChan chan []byte, log *glog.Glogger,
	detect *gnError.GnExceptionDetect) ILinker {
	rpcx := &RpcxLinker{
		CWChan:     outChan,
		serverAddr: address,
		serverID:   serverId,
		isRuning:   false,
		exDetect:   detect,
		// clientConMaps: make(map[string]*grpc.ClientConn, 1<<4), // grpc clients  serverId:grpcClient
		conf: config,
	}
	return rpcx
}

func (rl *RpcxLinker) SendMsg(router string, data []byte) {

}
func (rl *RpcxLinker) GetSubRounter() string {
	return ""
}
func (rl *RpcxLinker) Run() error {
	return nil
}
func (rl *RpcxLinker) GetConnection() interface{} {
	return nil
}
func (rl *RpcxLinker) Done() {

}
