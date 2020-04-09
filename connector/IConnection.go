package connector

import (
	"context"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/linker"
)

// client Connection  interface
type IConnection interface {
	SendMessage(data []byte)
	Run()
	Done()
	// for  read  or   write msg
	WriteMsg(ctx context.Context)
	ReadMsg(ctx context.Context)
	GetConnectionCid() string
	GetConnection() interface{}
	SetBindId(bindId string)
	GetBindId() string
}

// connector interface
type IConnector interface {
	Run() error
	Done()
	AddRouterRearEndHandler(serverType string, handler RouteRearEndHFunc)
	// set checkOrigin 跨域 func
	SetCheckOriginHandler(handler CheckOriginHFunc)
	// set verify Connect func
	SetVerifyConnectHandler(handler VerifyClientConnectHFunc)
	AddExceptionHandler(handler gnError.ExceptionHandleFunc)
	LoopClientReadChan(ctx context.Context)
	LoopLinkerChan(ctx context.Context)
	ListenAndRun() error
	GetLinker() linker.ILinker
	GetLoger() *glog.Glogger
	SendPack(serverAddress, router, bindId, cid string, data []byte)
	GetServerIdByRouter(serverType string, LogicBindId string, cid string, serverList []*config.ServersConfig) string
}

// 用户 接收消息   封装  channel 传输用
type ChanMsgPack struct {
	cid         string // 客户端连接  cid
	body        []byte
	logicBindID string // 逻辑层绑定Id
}
