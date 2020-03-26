package master

import (
	"runtime"

	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
)

type CmdHandlerFunc func(cmd, nodeId string, date []byte)
type TimeOutHandlerFunc func(list []NodeInfo)

type IMaster interface {
	Run() error
	AddHandler(cmd string, handler CmdHandlerFunc)
	SendCMD(cmd, nodeId string, data []byte) (result []byte, err error)
	TimeOutServerListListener(tHandler TimeOutHandlerFunc)
	GetRunTimeMemStats(nodeId string) (*runtime.MemStats, error)
	Done()
	GetNodeInfos() map[string]*NodeInfo
	AddExceptionHandler(handler gnError.ExceptionHandleFunc)
	GetLogger() *glog.Glogger
}
