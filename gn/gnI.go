package gn

import (
	"sync"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/linker"
)

type HandlerFunc func(IPack)

// 多goroutine  通道同喜   封装  channel 传输用
type ChanMsgPacket struct {
	serverId string // 服务器Id
	body     []byte //body
}

// router
type IRouter interface {
	APIRouter(router string, handlers ...HandlerFunc)
	RPCRouter(router string, handler HandlerFunc)
}

// pack  interface
type IPack interface {
	Abort()
	IsAbort() bool
	GetAPP() IApp
	GetData() []byte
	SetHandlersTranferObj(decodOjb interface{})
	GetHandlersTranferObj() interface{}
	GetSession() *Session
	GetRouter() string
	ResultJson(obj interface{})
	ExceptionAbortJson(code, msg string)
	ResultProtoBuf(obj interface{})
	ResultBytes(bytes []byte)
	GetResults() []byte
	GetReplyToken() string
	GetDstSubRouter() string
	GetSrcSubRouter() string
	GetLogger() *glog.Glogger
	GetBindId() string
	SetRPCRespCode(code int)
	GetRPCRespCode() int32
}

// IApp
type IApp interface {
	PushMsg(session *Session, data []byte)
	SendRPCMsg(serverId string, handlerName string, data []byte) (IPack, error)

	APIRouter(router string, handlers ...HandlerFunc)
	RPCRouter(router string, handler HandlerFunc)

	CMDHandler(cmd string, handler HandlerFunc)

	NewRouter() IRouter

	GetAPIRounterLock() *sync.RWMutex
	GetRPCRounterLock() *sync.RWMutex

	NewGroup(groupName string) *Group
	GetGroup(groupName string) *Group
	BoadCastByGroupName(groupName string, data []byte)

	SetObjectByTag(tag string, obj interface{})
	GetObjectByTag(tag string) (interface{}, bool)
	DelObjectByTag(tag string)

	callRPCHandlers(pack IPack)
	callAPIHandlers(pack IPack)

	Done()
	GetServerConfig() *config.Config
	GetLogger() *glog.Glogger
	Run() error

	AddExceptionHandler(handler gnError.ExceptionHandleFunc)
	GetLinker() linker.ILinker
	GetRunRoutineNum() int
	GetLoger() *glog.Glogger
}
