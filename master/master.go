package master

import (
	"context"
	"flag"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/linker"

	"github.com/wmyi/gn/config"

	"github.com/golang/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
)

var (
	masterOnce           sync.Once
	mInstance            *Master
	onceCMDRequestIdBase uint64 = 1000
	mode                 string
	jsonI                = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	NODE_NAME string = "master"
)

type Master struct {
	config           *config.Config
	links            linker.ILinker
	Logger           *glog.Glogger
	serverMap        map[string]*NodeInfo
	inChan           chan []byte
	isRunning        bool
	cmdHandlers      map[string]CmdHandlerFunc
	onceCmdHandlers  map[string]chan []byte
	timeoutHandler   TimeOutHandlerFunc
	loopCancal       context.CancelFunc
	handlesMutex     sync.RWMutex
	onceHandlesMutex sync.RWMutex
	exDetect         *gnError.GnExceptionDetect
}

func ParseCommands() {
	commands := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	commands.StringVar(&mode, "mode", "", " mode is debug  log is std.out  defalut is logs file ")
	commands.Parse(os.Args[1:])
}

func DefaultMaster(conf *config.Config) (IMaster, error) {
	ParseCommands()
	masterOnce.Do(func() {
		mInstance = &Master{
			config:          conf,
			serverMap:       make(map[string]*NodeInfo),
			inChan:          make(chan []byte, 1<<6),
			cmdHandlers:     make(map[string]CmdHandlerFunc, 1<<4),
			onceCmdHandlers: make(map[string]chan []byte, 1<<4),
			isRunning:       false,
		}

		masterConf := conf.GetMasterConf()
		mInstance.Logger = glog.NewLogger(conf, "master", mode)
		mInstance.exDetect = gnError.NewGnExceptionDetect(mInstance.Logger)
		if masterConf != nil {
			mInstance.links = linker.NewNatsClient(masterConf.ID, &conf.Natsconf, mInstance.inChan, mInstance.Logger,
				mInstance.exDetect)
		}

	})

	return mInstance, nil
}

func (m *Master) Run() error {
	if !m.isRunning {

		err := m.links.Run()
		if err != nil {
			m.Logger.Errorf("Master Run error  %v", err)
			return err
		}

		ctx, cancal := context.WithCancel(context.Background())
		m.loopCancal = cancal
		go m.loopNatsChanMsg(ctx)
		m.checkNotifyNodeInfo()
		m.isRunning = true
		m.exDetect.Run(false)
	}
	return nil
}

func (m *Master) PingHandler(cmd, nodeId string, date []byte) {

	if value, ok := m.serverMap[nodeId]; ok && value != nil {
		value.LastTime = value.CurrenTime
		value.CurrenTime = time.Now().UnixNano()
		value.DiffTime = time.Duration((value.CurrenTime - value.LastTime))

		nodeInfo := &config.CmdMsg{}
		err := proto.Unmarshal(date, nodeInfo)
		if err != nil {
			m.Logger.Errorf("master  receive CMD_PING  proto.Unmarshal error ", err)
			return
		}
		value.RoutineNum = nodeInfo.GetRunRoutineNum()
		m.Logger.Infof(" pingHandler  cmd  %s   nodeId  %s info  %v ", cmd, nodeId, value)
	}

}

func (m *Master) GetRunTimeMemStats(nodeId string) (*runtime.MemStats, error) {
	if len(nodeId) > 0 {
		result, err := m.SendCMD(config.CMD_MEM, nodeId, []byte(""))
		if err != nil {
			return nil, err
		}
		if len(result) > 0 {

			memSet := &runtime.MemStats{}
			err = jsonI.Unmarshal(result, memSet)
			if err != nil {
				return nil, err
			}
			return memSet, nil
		}

	}
	return nil, gnError.ErrMasterCMDMEM

}

func (m *Master) checkNotifyNodeInfo() {
	if len(m.serverMap) <= 0 {
		// add handler
		m.AddHandler(config.CMD_PING, m.PingHandler)
		// init nodeInfo  by  conf
		// connector
		for _, nodeConf := range m.config.Connector {
			if _, ok := m.serverMap[nodeConf.ID]; !ok {
				m.serverMap[nodeConf.ID] = &NodeInfo{
					NodeId: nodeConf.ID,
					Host:   nodeConf.Host,
					Port:   nodeConf.ClientPort,
				}

				pack := m.newCMDPack(NODE_NAME, nodeConf.ID, config.CMD_PING, nil, "")
				out, err := proto.Marshal(pack)
				if err == nil {
					m.Logger.Infof("master  checkNotifyNodeInfo send   serverAddress     %v   Msg  %v \n ", nodeConf.ID, out)
					m.links.SendMsg(nodeConf.ID, out)
				} else {
					m.Logger.Errorf("master receive  pb Marshal  errr     %v  ", err)
				}

			}

		}
		// node servers
		for _, nodeConf := range m.config.Servers {
			if _, ok := m.serverMap[nodeConf.ID]; !ok {
				m.serverMap[nodeConf.ID] = &NodeInfo{
					NodeId: nodeConf.ID,
				}
				pack := m.newCMDPack(NODE_NAME, nodeConf.ID, config.CMD_PING, nil, "")
				out, err := proto.Marshal(pack)
				if err == nil {
					m.Logger.Infof("master   send  init serverAddress     %v   Msg  %v \n ", nodeConf.ID, out)
					m.links.SendMsg(nodeConf.ID, out)
				} else {
					m.Logger.Errorf("master receive  pb Marshal  errr     %v  ", err)
				}

			}

		}
	}
}

func (m *Master) loopNatsChanMsg(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			m.Logger.Errorf("master   Read stop error ", string(debug.Stack()))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(m.config.MasterConf.NodeHeartBeart) * time.Second):
			m.checkNodeTimeOut()
			break
		case data, ok := <-m.inChan:
			if !ok {
				return
			} else {
				dTsession := &config.TSession{}
				err := proto.Unmarshal(data, dTsession)
				if err != nil {
					m.Logger.Infof("  master in receive msg  UnMarsha err   %v ", err)
				}

				if len(dTsession.GetRouter()) > 0 && len(dTsession.GetSrcSubRouter()) > 0 {
					if value, ok := m.serverMap[dTsession.GetSrcSubRouter()]; ok && value != nil {
						m.handlePack(dTsession)
					} else {
						break
					}
				}
				break
			}
		}
	}

}

func (m *Master) newCMDPack(srcNodeId, destNodeId, cmd string, data []byte, requestId string) *config.TSession {
	pack := &config.TSession{
		SrcSubRouter: srcNodeId,
		DstSubRouter: destNodeId,
		St:           config.TSession_CMD,
		Router:       cmd,
	}
	if data != nil && len(data) > 0 {
		pack.Body = data
	}
	if len(requestId) > 0 {
		pack.ReplyToken = requestId
	}
	return pack
}

func (m *Master) handlePack(ts *config.TSession) {
	cmd := ts.GetRouter()
	if handle, ok := m.cmdHandlers[cmd]; ok && handle != nil {
		handle(cmd, ts.GetSrcSubRouter(), ts.GetBody())
	}
	replyToke := ts.GetReplyToken()
	if msgChan, ok := m.onceCmdHandlers[replyToke]; len(replyToke) > 0 && ok && msgChan != nil {
		if len(ts.GetBody()) > 0 {
			msgChan <- ts.GetBody()
		} else {
		}
	}
}

func (m *Master) AddHandler(cmd string, handler CmdHandlerFunc) {
	if len(cmd) > 0 && handler != nil {
		m.handlesMutex.Lock()
		m.cmdHandlers[cmd] = handler
		m.handlesMutex.Unlock()
	}
}
func (m *Master) SendCMD(cmd, nodeId string, data []byte) (result []byte, err error) {
	if len(cmd) > 0 && len(nodeId) > 0 {
		requestId := cmd + "-" + strconv.FormatUint(atomic.AddUint64(&onceCMDRequestIdBase, 1), 10)
		msgChan := make(chan []byte)
		m.onceHandlesMutex.Lock()
		m.onceCmdHandlers[requestId] = msgChan
		m.onceHandlesMutex.Unlock()
		pack := m.newCMDPack(NODE_NAME, nodeId, cmd, data, requestId)
		out, err := proto.Marshal(pack)
		if err == nil {
			m.Logger.Infof("master  send ping  serverAddress     %v   Msg  %v \n ", nodeId, out)
			m.links.SendMsg(nodeId, out)
		} else {
			m.Logger.Errorf("master send ping recve  pb Marshal  errr     %v  ", err)
		}

		var timed = time.NewTimer(time.Duration(30) * time.Second)
		var result []byte
		var ok bool
		select {
		case result, ok = <-msgChan:
			m.onceHandlesMutex.Lock()
			delete(m.onceCmdHandlers, requestId)
			m.onceHandlesMutex.Unlock()
			if !ok {
				return nil, gnError.ErrCHanError
			}
			break
		case <-timed.C:
			m.onceHandlesMutex.Lock()
			delete(m.onceCmdHandlers, requestId)
			m.onceHandlesMutex.Unlock()
			return nil, gnError.ErrRPCHandleTimeOut
		}
		return result, nil
	} else {
		return nil, gnError.ErrParameter
	}
}

func (m *Master) TimeOutServerListListener(tHandler TimeOutHandlerFunc) {
	m.timeoutHandler = tHandler
}

func (m *Master) checkNodeTimeOut() {
	now := time.Now().UnixNano()
	nodeSlices := make([]NodeInfo, 1<<1)
	for ID, info := range m.serverMap {
		// now - last pong time  > heartBeartTime
		if time.Duration(now-info.CurrenTime).Seconds() > float64(m.config.MasterConf.NodeHeartBeart) {
			nodeSlices = append(nodeSlices, *info)
		}
		pack := m.newCMDPack(NODE_NAME, ID, config.CMD_PING, nil, "")
		out, err := proto.Marshal(pack)
		if err == nil {
			m.Logger.Infof("master  send ping  serverAddress     %v   Msg  %v \n ", ID, out)
			m.links.SendMsg(ID, out)
		} else {
			m.Logger.Errorf("master send ping recve  pb Marshal  errr     %v  ", err)
		}
	}

	if m.timeoutHandler != nil {
		m.timeoutHandler(nodeSlices)
	}
}

func (m *Master) Done() {
	if m.loopCancal != nil {
		m.loopCancal()
	}
	if m.isRunning {
		m.isRunning = false
	}

}

func (m *Master) GetLogger() *glog.Glogger {
	return m.Logger
}

func (m *Master) AddExceptionHandler(handler gnError.ExceptionHandleFunc) {
	if handler != nil && m.exDetect != nil {
		m.exDetect.AddExceptionHandler(handler)
	}
}

func (m *Master) GetNodeInfos() map[string]*NodeInfo {
	return m.serverMap
}

type NodeInfo struct {
	NodeId     string
	LastTime   int64
	CurrenTime int64
	DiffTime   time.Duration
	RoutineNum int64
	Host       string
	Port       int
}
