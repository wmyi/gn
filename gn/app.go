package gn

import (
	"context"
	"flag"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/linker"

	"github.com/golang/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
)

var (
	instance *App
	appOnce  sync.Once

	jsonI              = jsoniter.ConfigCompatibleWithStandardLibrary
	ReTokenBase uint64 = 1
	serverId    string
	mode        string
)

type App struct {
	config         *config.Config
	apiRouters     map[string][]HandlerFunc
	rpcRouters     map[string][]HandlerFunc
	links          linker.ILinker
	groups         map[string]*Group
	outChan        chan *config.TSession
	inChan         chan []byte
	rpcRespMap     map[string]chan IPack
	rpcHandleMutex sync.RWMutex
	apiRouterMutex sync.RWMutex
	rpcRouterMutex sync.RWMutex
	handleTimeOut  int
	rpcTimeOut     int
	exDetect       *gnError.GnExceptionDetect
	logger         *glog.Glogger
	cmdMaster      IAppCmd
	isRuning       bool
	RRoutineCan    context.CancelFunc
	WRoutineCan    context.CancelFunc
	runRoutineChan chan bool
	maxRoutineNum  int
	tagObjs        *sync.Map
}

func ParseCommands() {
	commands := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	commands.StringVar(&serverId, "serverId", "", " server id   logic server  who  ")
	commands.StringVar(&mode, "mode", "", " mode is debug  log is std.out  defalut is logs file ")
	commands.Parse(os.Args[1:])
}

func DefaultApp(conf *config.Config) (IApp, error) {
	ParseCommands()
	if len(serverId) <= 0 {
		return nil, gnError.ErrNoServerIdPar
	}
	appOnce.Do(func() {
		instance = &App{
			config:        conf,
			apiRouters:    make(map[string][]HandlerFunc, 1<<7),
			rpcRouters:    make(map[string][]HandlerFunc, 1<<7),
			outChan:       make(chan *config.TSession, 1<<10),
			inChan:        make(chan []byte, 1<<10),
			rpcRespMap:    make(map[string]chan IPack, 1<<10),
			handleTimeOut: 10,
			rpcTimeOut:    5,
			isRuning:      false,
			maxRoutineNum: 1024, // defalut maxRoutine 同时
			tagObjs:       &sync.Map{},
		}
		serverConfig := conf.GetServerConfByServerId(serverId)
		// timeout
		if serverConfig.RPCTimeOut != 0 {
			instance.rpcTimeOut = serverConfig.RPCTimeOut
		}

		if serverConfig.HandleTimeOut != 0 {
			instance.handleTimeOut = serverConfig.HandleTimeOut
		}

		instance.logger = glog.NewLogger(conf, serverId, mode)
		instance.exDetect = gnError.NewGnExceptionDetect(instance.logger)

		if serverConfig != nil {
			instance.links = linker.NewNatsClient(serverConfig.ID, &conf.Natsconf, instance.inChan,
				instance.logger, instance.exDetect)
			instance.handleTimeOut = serverConfig.HandleTimeOut
			instance.rpcTimeOut = serverConfig.RPCTimeOut
			instance.maxRoutineNum = serverConfig.MaxRunRoutineNum
		}
		instance.cmdMaster = NewAppCmd(instance.logger, instance.links, instance.exDetect)
	})
	return instance, nil
}

func (a *App) SetObjectByTag(tag string, obj interface{}) {
	if len(tag) > 0 && obj != nil {
		a.tagObjs.Store(tag, obj)
	}
}
func (a *App) GetObjectByTag(tag string) interface{} {
	if len(tag) > 0 {
		if obj, ok := a.tagObjs.Load(tag); ok {
			return obj
		}
	}
	return nil
}

func (a *App) DelObjectByTag(tag string) {
	if len(tag) > 0 {
		a.tagObjs.Delete(tag)
	}
}

func (a *App) GetLogger() *glog.Glogger {
	return a.logger
}

func (a *App) GetAPIRounterLock() *sync.RWMutex {
	return &a.apiRouterMutex
}

func (a *App) GetRPCRounterLock() *sync.RWMutex {
	return &a.rpcRouterMutex
}

func (a *App) Run() error {

	if !a.isRuning {
		a.isRuning = true
		// init max routine channel
		if a.runRoutineChan == nil && a.maxRoutineNum >= 0 {
			a.runRoutineChan = make(chan bool, a.maxRoutineNum)
		} else {
			return gnError.ErrConfigLackRoutineNum
		}

		err := a.links.Run()
		if err != nil {
			a.logger.Errorf("app natsLinker Run error  %v", err)
			return err
		}
		err = a.cmdMaster.Run()
		if err != nil {
			a.logger.Errorf("app cmdApp Run error  %v", err)
			return err
		}
		rctx, rcancal := context.WithCancel(context.Background())
		go a.loopReadChanMsg(rctx)
		a.RRoutineCan = rcancal
		wctx, wcancal := context.WithCancel(context.Background())
		go a.loopWriteChanMsg(wctx)
		a.WRoutineCan = wcancal
		a.exDetect.Run(false)
		return nil
	} else {
		a.logger.Errorf("App is aleady Runing ")
		return gnError.ErrAPPRuning
	}

}

func (a *App) GetServerConfig() *config.Config {
	return a.config
}

func (a *App) BoadCastByGroupName(groupName string, data []byte) {
	group := a.GetGroup(groupName)
	if group != nil {
		group.BoadCast(data)
	}
}

func (a *App) PushMsg(session *Session, data []byte) {
	if len(session.GetSrcSubRouter()) > 0 && len(data) > 0 {
		pack := &config.TSession{
			Cid:          session.GetCid(),
			SrcSubRouter: a.links.GetSubRounter(),
			DstSubRouter: session.GetSrcSubRouter(),
			Body:         data,
			St:           config.TSession_LOGIC,
			LogicBindId:  session.GetBindId(),
		}
		if a.outChan != nil {
			a.outChan <- pack
		}
	}
}
func (a *App) SendRPCMsg(serverId string, handlerName string, data []byte) (IPack, error) {
	if len(serverId) > 0 && len(handlerName) > 0 && len(data) > 0 {
		token := serverId + "-" + strconv.FormatUint(atomic.AddUint64(&ReTokenBase, 1), 10)
		msgChan := make(chan IPack, 1)
		a.rpcHandleMutex.Lock()
		a.rpcRespMap[token] = msgChan
		a.rpcHandleMutex.Unlock()
		// 发送 数据
		pack := &config.TSession{
			SrcSubRouter: a.links.GetSubRounter(),
			DstSubRouter: serverId,
			Body:         data,
			St:           config.TSession_LOGIC,
			ReplyToken:   token,
			Router:       handlerName,
		}
		if a.outChan != nil {
			a.outChan <- pack
		}
		// 线程 阻塞  等待  消息  然后 直接返回
		var ok bool
		var data IPack
		var timed = time.NewTimer(time.Duration(a.rpcTimeOut) * time.Second)
		select {
		case data, ok = <-msgChan:
			a.rpcHandleMutex.Lock()
			delete(a.rpcRespMap, token)
			a.rpcHandleMutex.Unlock()
			if !ok {
				return nil, gnError.ErrCHanError
			}
		case <-timed.C:
			a.rpcHandleMutex.Lock()
			delete(a.rpcRespMap, token)
			a.rpcHandleMutex.Unlock()
			return nil, gnError.ErrRPCHandleTimeOut
		}
		return data, nil
	} else {
		return nil, gnError.ErrRPCParameter
	}
}

func (a *App) loopWriteChanMsg(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("App write Routine panic ", string(debug.Stack()))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-a.outChan:
			if ok {
				// parse msg to  nats   or  end server
				a.logger.Infof(" app  receive message bindId %s   msg:  %v ", data.GetLogicBindId(), string(data.GetBody()))
				if len(data.GetDstSubRouter()) > 0 {
					// push
					out, err := proto.Marshal(data)
					if err == nil {
						a.links.SendMsg(data.GetDstSubRouter(), out)
					} else {
						a.logger.Errorf(" app  receive message pb Marshal err %v ", err)
					}
				}
			}
		}
	}

}

func (a *App) GetRunRoutineNum() int {
	if a.runRoutineChan != nil {
		return len(a.runRoutineChan)
	}
	return 0
}

func (a *App) decodeFrontPack(data []byte) {

	dTsession := &config.TSession{}

	err := proto.Unmarshal(data, dTsession)
	a.logger.Infof(" app  in  reveice msg      %v ", string(data))
	if err != nil {
		a.logger.Errorf("  app in revice pb msg   UnMarsha err   %v ", err)
		a.exDetect.ThrowException(&gnError.GnException{
			Id:        "",
			Exception: gnError.PB_UMARSHAL_ERROR,
			Msg:       gnError.ErrAUnmarshalPbPack.Error(),
		})
		return
	}
	pack := NewPack(a, dTsession, NewSession(dTsession.GetCid(), dTsession.GetSrcSubRouter(), dTsession.GetLogicBindId()))
	if dTsession.GetSt() == config.TSession_CMD {
		a.cmdMaster.ReceiveCmdPack(pack)
	} else if dTsession.GetSt() == config.TSession_CONNECTOR {
		if len(pack.GetSession().GetCid()) > 0 {

			// runtine number +1 if block wait
			a.runRoutineChan <- true
			go a.callAPIHandlers(pack)
		}
	} else if dTsession.GetSt() == config.TSession_LOGIC {
		if len(pack.GetReplyToken()) > 0 {
			// runtine number +1 if block wait
			a.runRoutineChan <- true
			go a.callRPCHandlers(pack)
		}
	}
}

func (a *App) loopReadChanMsg(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("App Read Routine panic ", string(debug.Stack()))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-a.inChan:
			if ok {
				a.decodeFrontPack(data)
			}
		}
	}
}

func (a *App) Done() {
	if a.isRuning {
		a.isRuning = false
		if a.RRoutineCan != nil {
			a.RRoutineCan()
		}
		if a.WRoutineCan != nil {
			a.WRoutineCan()
		}
		if a.links != nil {
			a.links.Done()
		}

		if a.cmdMaster != nil {
			a.cmdMaster.Done()
		}
		if a.exDetect != nil {
			a.exDetect.Done()
		}
		if a.logger != nil {
			a.logger.Done()
		}

	}
}

func (a *App) callRPCHandlers(pack IPack) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("App RPC receive IPack routine panic ", string(debug.Stack()))
		}
		// runtine number -1
		<-a.runRoutineChan
	}()

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(a.handleTimeOut)*time.Second)
	endChan := make(chan bool)
	go func(ctx context.Context, finishChan chan<- bool) {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Errorf("App handle RPC callback  routine panic ", string(debug.Stack()))
			}
			// runtine number -1
			<-a.runRoutineChan
			finishChan <- true // 完成
		}()
		// runtine number +1
		a.runRoutineChan <- true
		if len(pack.GetRouter()) == 0 && len(pack.GetReplyToken()) > 0 {
			a.rpcHandleMutex.Lock()
			respChan, ok := a.rpcRespMap[pack.GetReplyToken()]
			if ok && respChan != nil {
				delete(a.rpcRespMap, pack.GetReplyToken())
			}
			a.rpcHandleMutex.Unlock()
			if respChan != nil {
				respChan <- pack
			} else {
				a.logger.Errorf("App handle RPC Respon callback  routine  %v ", pack)
			}
		} else if len(pack.GetRouter()) > 0 {
			a.rpcRouterMutex.RLock()
			handlers, ok := a.rpcRouters[pack.GetRouter()]
			a.rpcRouterMutex.RUnlock()
			if len(handlers) > 0 && ok {
				for _, handler := range handlers {
					handler(pack)
					if pack.IsAbort() {
						break
					}
				}

				if pack.GetResults() != nil && len(pack.GetResults()) > 0 {
					replyPack := &config.TSession{
						Cid:          pack.GetSession().GetCid(),
						SrcSubRouter: pack.GetDstSubRouter(),
						DstSubRouter: pack.GetSrcSubRouter(),
						Body:         pack.GetResults(),
						St:           config.TSession_LOGIC,
						ReplyToken:   pack.GetReplyToken(),
					}
					if a.outChan != nil {
						a.outChan <- replyPack
					}

				}

			} else {
				a.logger.Errorf("App  RPCHandlers NO  router  %s   please add  handler", pack.GetRouter())
			}
		}
	}(ctx, endChan)
	select {
	case <-ctx.Done():
		panic(gnError.ErrRPCHandleTimeOut)
	case <-endChan:
		close(endChan)
		return
	}
}

func (a *App) callAPIHandlers(pack IPack) {

	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("App receive IPack routine panic ", string(debug.Stack()))
		}
		// runtine number -1
		<-a.runRoutineChan
	}()

	endChan := make(chan bool, 1)
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(a.handleTimeOut)*time.Second)
	go func(ctx context.Context, finishChan chan<- bool) {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Errorf("App handle api callback  routine panic ", string(debug.Stack()))
			}
			// runtine number -1
			<-a.runRoutineChan
			finishChan <- true
		}()
		// runtine number +1
		a.runRoutineChan <- true
		if len(pack.GetRouter()) > 0 {
			a.apiRouterMutex.RLock()
			handlers := a.apiRouters[pack.GetRouter()]
			a.apiRouterMutex.RUnlock()
			if len(handlers) > 0 {
				//  handler
				for _, handler := range handlers {
					handler(pack)
					if pack.IsAbort() {
						break
					}
				}
				//  next results send  link
				if pack.GetResults() != nil && len(pack.GetResults()) > 0 {
					replyPack := &config.TSession{
						Cid:          pack.GetSession().GetCid(),
						SrcSubRouter: pack.GetDstSubRouter(),
						DstSubRouter: pack.GetSrcSubRouter(),
						Body:         pack.GetResults(),
						St:           config.TSession_LOGIC,
						LogicBindId:  pack.GetSession().GetBindId(),
					}
					if a.outChan != nil {
						a.outChan <- replyPack
					}
				}
			} else {
				a.logger.Errorf("App API RECALL  router  %s  handler is nil ", pack.GetRouter())
				a.ErrorToConnector(pack.GetSession(), gnError.ErrAPIHandleTimeOutCode, gnError.ErrAPINOHandle.Error())
			}
		}

	}(ctx, endChan)
	select {
	case <-endChan:
		// finish
		close(endChan)
		return
	case <-ctx.Done():
		// timeOut
		a.logger.Errorf("App callAPIHandlers   timeout  %v ", pack)
		a.ErrorToConnector(pack.GetSession(), gnError.ErrAPIHandleTimeOutCode, gnError.ErrAPIHandleTimeOut.Error())
		panic(gnError.ErrAPIHandleTimeOut)
	}
}

func (a *App) CMDHandler(cmd string, handler HandlerFunc) {
	if a.cmdMaster != nil {
		a.cmdMaster.AddCmdHandler(cmd, handler)
	}
}

func (a *App) ErrorToConnector(session *Session, code string, errorMsg string) {
	packResponse := &gnError.PackError{
		Code:     code,
		ErrorMsg: errorMsg,
	}

	out, err := jsonI.Marshal(packResponse)
	if err != nil {
		a.logger.Errorf("app ErrorToConnector  jsonI.Marshal  err  ", err)
		return
	}
	a.PushMsg(session, out)

}

func (a *App) APIRouter(router string, handlers ...HandlerFunc) {
	if a.apiRouters != nil {
		a.apiRouterMutex.Lock()
		if a.apiRouters[router] == nil {
			a.apiRouters[router] = make([]HandlerFunc, 0, 10)
		}
		a.apiRouters[router] = append(a.apiRouters[router], handlers...)
		a.apiRouterMutex.Unlock()
	}
}
func (a *App) RPCRouter(router string, handlers HandlerFunc) {
	if a.rpcRouters != nil {
		a.rpcHandleMutex.Lock()
		if a.rpcRouters[router] == nil {
			a.rpcRouters[router] = make([]HandlerFunc, 0, 10)
		}
		a.rpcRouters[router] = append(a.rpcRouters[router], handlers)
		a.rpcHandleMutex.Unlock()
	}
}

func (a *App) NewRouter() IRouter {
	return &Router{
		app:        a,
		apiRouters: a.apiRouters,
		rpcRouters: a.rpcRouters,
	}
}

func (a *App) NewGroup(groupName string) *Group {
	var group = NewGroup(a, groupName)
	if a.groups == nil {
		a.groups = make(map[string]*Group)
	}
	a.groups[groupName] = group
	return group
}

func (a *App) GetGroup(groupName string) *Group {
	if a.groups != nil {
		return a.groups[groupName]
	}
	return nil
}

func (a *App) AddExceptionHandler(handler gnError.ExceptionHandleFunc) {
	if handler != nil && a.exDetect != nil {
		a.exDetect.AddExceptionHandler(handler)
	}
}

func (a *App) GetLinker() linker.ILinker {
	return a.links
}

func (a *App) GetLoger() *glog.Glogger {
	return a.logger
}
