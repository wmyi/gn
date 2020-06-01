package gn

import (
	"github.com/spf13/viper"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/linker"
)

type HandlerFunc func(IPack)

// 多goroutine  通道同喜   封装  channel 传输用
type ChanMsgPacket struct {
	serverId string // 服务器Id
	body     []byte //body
}

// handler  router 注册
type Handler struct {
	Funcs      []HandlerFunc
	NewRoutine bool
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
	GetBindId() string
	SetRPCRespCode(code int)
	GetRPCRespCode() int32
	GetContextValue(key string) interface{}
	SetContextValue(key string, value interface{})
}

// IApp
type IApp interface {
	PushMsg(session *Session, data []byte)
	PushJsonMsg(session *Session, obj interface{})
	PushProtoBufMsg(session *Session, obj interface{})

	NotifyRPCMsg(serverId string, handlerName string, data []byte) error
	NotifyRPCJsonMsg(serverId string, handlerName string, obj interface{}) error
	NotifyRPCProtoBufMsg(serverId string, handlerName string, obj interface{}) error
	RequestRPCMsg(serverId string, handlerName string, data []byte) (IPack, error)
	RequestRPCJsonMsg(serverId string, handlerName string, obj interface{}) (IPack, error)
	RequestRPCProtoBufMsg(serverId string, handlerName string, obj interface{}) (IPack, error)

	APIRouter(router string, newGoRoutine bool, handlerFunc ...HandlerFunc)
	RPCRouter(router string, newGoRoutine bool, handlerFunc HandlerFunc)

	CMDHandler(cmd string, handler HandlerFunc)

	NewGroup(groupName string) *Group
	GetGroup(groupName string) (*Group, bool)
	BoadCastByGroupName(groupName string, data []byte)
	DelGroup(groupName string)

	SetObjectByTag(tag string, obj interface{})
	GetObjectByTag(tag string) (interface{}, bool)
	DelObjectByTag(tag string)

	callRPCHandlers(pack IPack)
	callAPIHandlers(pack IPack)

	Done()
	GetServerConfig() *config.Config
	Run() error

	AddExceptionHandler(handler gnError.ExceptionHandleFunc)
	GetLinker() linker.ILinker
	GetRunRoutineNum() int
	AddConfigFile(keyName, path, configType string) error
	GetConfigViper(keyName string) *viper.Viper
	UseMiddleWare(middleWare ...GNMiddleWare)
}

type GNMiddleWare interface {
	Before(IPack)
	After(IPack)
}
