package connector

import (
	"context"
	"gn/glog"
	"gn/gnError"
	"gn/linker"
)

// client Connection  interface
type IConnection interface {
	SendMessage(data []byte)
	Run()
	Done()
	// for  read  or   write msg
	WriteMsg()
	ReadMsg()
	GetConnectionCid() string
	GetConnection() interface{}
	SetBindId(bindId string)
	GetBindId() string
	addRouterRearEndHandler(serverType string, handler RouteRearEndHFunc)
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
}

// 用户 接收消息   封装  channel 传输用
type ChanMsgPack struct {
	cid         string // 客户端连接  cid
	body        []byte
	logicBindID string // 逻辑层绑定Id
}
