package linker

import (
	"net"
	"time"
)

// rpc NodeConnection
type NodeConnection struct {
	Con net.Conn

	// ReadTimeout sets readdeadline for underlying net.Conns
	ReadTimeout time.Duration
	// WriteTimeout sets writedeadline for underlying net.Conns
	WriteTimeout       time.Duration
	closing            bool // user has called Close
	isClientConnection bool
	remoteNodeId       string
	isRuning           bool
	CWChan             chan []byte
}

func NewNodeConnection(con net.Conn, isClientCon bool, remoteNodeId string, outChan chan []byte) *NodeConnection {
	return &NodeConnection{
		Con:                con,
		closing:            false,
		isClientConnection: isClientCon,
		remoteNodeId:       remoteNodeId,
		isRuning:           false,
		CWChan:             outChan,
	}
}

func (nc *NodeConnection) getRemoteNodeId() string {
	return nc.remoteNodeId
}

func (nc *NodeConnection) ReadByte() {

}

func (nc *NodeConnection) Write(nodeId string, data []byte) {
	if nc.Con != nil && nc.isRuning && nc.remoteNodeId == nodeId {
		// nc.Con.Write(data)
	}
}

func (nc *NodeConnection) Run() {
	if !nc.isRuning {
		nc.isRuning = true
		// new goroutine run  read
		// go routine readByte
	}
}
