package linker

import (
	"net"
	"time"

	logger "github.com/wmyi/gn/glog"
)

// Server rpc server  user tcp

type RpcServer struct {
	ln           net.Listener
	readTimeout  time.Duration
	writeTimeout time.Duration
	serverId     string
	address      string
	isRuning     bool
}

func NewRpcServer(serverId, address string) *RpcServer {
	return &RpcServer{
		serverId: serverId,
		address:  address,
		isRuning: false,
	}
}
func (rs *RpcServer) GetAddress() string {
	return rs.address
}

func (rs *RpcServer) GetServerId() string {
	return rs.serverId
}

func (rs *RpcServer) RunListen() error {
	if !rs.isRuning {
		rs.isRuning = true
		if rs.ln == nil {
			ln, err := net.Listen("tcp", rs.address)
			if err != nil {
				return err
			}
			for {
				conn, err := ln.Accept()
				if err != nil {
					logger.Errorf("rpc server StartListen accept error: %v   \n", err)
					continue
				}

			}

			defer ln.Close()
			rs.ln = ln
		}
	}

	return nil
}
