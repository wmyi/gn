package linker

import (
	"net"
	"runtime/debug"
	"sync"
	"time"

	"github.com/wmyi/gn/config"
	logger "github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
)

var (
	pendPackMapMux sync.RWMutex
)

type RpcLinker struct {
	CWChan     chan []byte
	serverAddr string
	serverID   string
	isRuning   bool
	//ConnectTimeout sets timeout for dialing
	ConnectTimeout  time.Duration
	exDetect        *gnError.GnExceptionDetect
	conf            *config.Config
	NodeConMaps     map[string]*NodeConnection
	NodeServer      *RpcServer
	pendingPackMaps map[string][][]byte
}

func NewRpcLinker(serverId string, config *config.Config, address string, outChan chan []byte,
	detect *gnError.GnExceptionDetect) ILinker {
	rpc := &RpcLinker{
		CWChan:          outChan,
		serverAddr:      address,
		serverID:        serverId,
		isRuning:        false,
		exDetect:        detect,
		NodeConMaps:     make(map[string]*NodeConnection, 1<<4), // rpc nodeConnection  serverId:nodeConnection
		conf:            config,
		pendingPackMaps: make(map[string][][]byte),
		ConnectTimeout:  time.Second * 10,
	}
	rpc.NodeServer = NewRpcServer(serverId, address)
	return rpc
}

func (rl *RpcLinker) SendPendingPack() {

}

func (rl *RpcLinker) SendMsg(router string, data []byte) {
	if len(router) > 0 && len(data) > 0 {
		if con, ok := rl.NodeConMaps[router]; ok && con != nil {
			con.Write(router, data)
		} else {
			// connect remote connection
			// add pending map
			if slicePack, ok := rl.pendingPackMaps[router]; ok && slicePack != nil {

			}
			go rl.ConnectRometeServer(router)
		}
	}
}

func (rl *RpcLinker) ConnectRometeServer(serverId string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("rpcLinker ConnectRometeServer Routine  ", string(debug.Stack()))
		}
	}()
	// get address

	// connect
	conn, err := net.DialTimeout("tcp", serverId, rl.ConnectTimeout)
	if err != nil {

		logger.Errorf("rpcLinker ConnectRometeServer connectTimeout   ", string(debug.Stack()))
		return
	}

}

func (rl *RpcLinker) GetSubRounter() string {
	return rl.serverID
}

func (rl *RpcLinker) Run() error {
	if !rl.isRuning {
		rl.isRuning = true
		if rl.NodeServer != nil {
			// new go routine  listen server
			err := rl.NodeServer.RunListen()
			if err != nil {
				return err
			}
		}

	}
	return nil
}
func (rl *RpcLinker) GetConnection() interface{} {
	return nil
}
func (rl *RpcLinker) Done() {

}
