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

	"github.com/spf13/viper"
	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/gnutil"
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
	apiRouters     map[string]*Handler
	rpcRouters     map[string]*Handler
	links          linker.ILinker
	groups         *sync.Map
	outChan        chan *config.TSession
	inChan         chan []byte
	rpcRespMap     map[string]chan IPack
	rpcHandleMutex sync.RWMutex
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
	vipers         map[string]*viper.Viper
	middlerWares   []GNMiddleWare
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
			apiRouters:    make(map[string]*Handler, 1<<7),
			rpcRouters:    make(map[string]*Handler, 1<<7),
			outChan:       make(chan *config.TSession, 1<<10),
			inChan:        make(chan []byte, 1<<10),
			rpcRespMap:    make(map[string]chan IPack, 1<<10),
			handleTimeOut: 10,
			rpcTimeOut:    5,
			isRuning:      false,
			maxRoutineNum: 1024, // defalut maxRoutine 同时
			tagObjs:       new(sync.Map),
			vipers:        make(map[string]*viper.Viper, 1<<4),
			middlerWares:  make([]GNMiddleWare, 0, 1<<3),
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
func (a *App) GetObjectByTag(tag string) (interface{}, bool) {
	if len(tag) > 0 {
		if obj, ok := a.tagObjs.Load(tag); ok {
			return obj, ok
		}
	}
	return nil, false
}

func (a *App) GetConfigViper(keyName string) *viper.Viper {
	return a.vipers[keyName]
}
func (a *App) AddConfigFile(keyName, path, configType string) error {
	if len(keyName) > 0 && len(path) > 0 {
		viper.SetConfigName(keyName)
		viper.SetConfigType(configType)
		viper.AddConfigPath(path)
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
		file := viper.GetViper()
		a.vipers[keyName] = file
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
	group, ok := a.GetGroup(groupName)
	if ok && group != nil {
		group.BroadCast(data)
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

func (a *App) PushJsonMsg(session *Session, obj interface{}) {
	if session != nil && obj != nil {
		out, ok := gnutil.JsonToBytes(obj, a.logger)
		if ok && out != nil {
			a.PushMsg(session, out)
		}
	}

}
func (a *App) PushProtoBufMsg(session *Session, obj interface{}) {
	if session != nil && obj != nil {
		out, ok := gnutil.ProtoBufToBytes(obj, a.logger)
		if ok && out != nil {
			a.PushMsg(session, out)
		}
	}

}

func (a *App) NotifyRPCMsg(serverId string, handlerName string, data []byte) error {
	if len(serverId) > 0 && len(handlerName) > 0 {
		// send msg
		// notify no set  ReplyToken  来区分 不 回传消息报
		pack := &config.TSession{
			SrcSubRouter: a.links.GetSubRounter(),
			DstSubRouter: serverId,
			Body:         data,
			St:           config.TSession_LOGIC,
			Router:       handlerName,
		}
		if a.outChan != nil {
			a.outChan <- pack
		}
		return nil
	}
	return gnError.ErrRPCParameter

}
func (a *App) NotifyRPCJsonMsg(serverId string, handlerName string, obj interface{}) error {
	out, ok := gnutil.JsonToBytes(obj, a.logger)
	if ok {
		return a.NotifyRPCMsg(serverId, handlerName, out)
	}
	return gnError.ErrRPCParameter
}
func (a *App) NotifyRPCProtoBufMsg(serverId string, handlerName string, obj interface{}) error {
	out, ok := gnutil.ProtoBufToBytes(obj, a.logger)
	if ok {
		return a.NotifyRPCMsg(serverId, handlerName, out)
	}

	return gnError.ErrRPCParameter
}

func (a *App) RequestRPCMsg(serverId string, handlerName string, data []byte) (IPack, error) {
	if len(serverId) > 0 && len(handlerName) > 0 {
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

func (a *App) RequestRPCJsonMsg(serverId string, handlerName string, obj interface{}) (IPack, error) {
	out, ok := gnutil.JsonToBytes(obj, a.logger)
	if ok {
		return a.RequestRPCMsg(serverId, handlerName, out)
	}
	return nil, gnError.ErrRPCParameter
}
func (a *App) RequestRPCProtoBufMsg(serverId string, handlerName string, obj interface{}) (IPack, error) {
	out, ok := gnutil.ProtoBufToBytes(obj, a.logger)
	if ok {
		return a.RequestRPCMsg(serverId, handlerName, out)
	}

	return nil, gnError.ErrRPCParameter
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
				a.logger.Infof(" app  Send linker message bindId %s   msg:  %v ", data.GetLogicBindId(), string(data.GetBody()))
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
			a.callAPIHandlers(pack)
		}
	} else if dTsession.GetSt() == config.TSession_LOGIC {
		if len(pack.GetSrcSubRouter()) > 0 && len(pack.GetDstSubRouter()) > 0 {
			a.callRPCHandlers(pack)
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
func (a *App) rpcGoRouinteHandler(pack IPack, handlerFuncs []HandlerFunc) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("App RPC receive IPack routine panic ", string(debug.Stack()))
		}
		// runtine number -1
		<-a.runRoutineChan
	}()

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(a.handleTimeOut)*time.Second)
	endChan := make(chan bool)
	go func(ctx context.Context, finishChan chan<- bool, handlerFuncs []HandlerFunc, pack IPack) {
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
		a.rangeHandler(pack, handlerFuncs)
		a.rpcResultPack(pack)
	}(ctx, endChan, handlerFuncs, pack)
	select {
	case <-ctx.Done():
		panic(gnError.ErrRPCHandleTimeOut)
	case <-endChan:
		close(endChan)
		return
	}
}

func (a *App) rpcResultPack(pack IPack) {
	if len(pack.GetSrcSubRouter()) > 0 && len(pack.GetDstSubRouter()) > 0 &&
		len(pack.GetReplyToken()) > 0 {
		replyPack := &config.TSession{
			Cid:          pack.GetSession().GetCid(),
			SrcSubRouter: pack.GetDstSubRouter(),
			DstSubRouter: pack.GetSrcSubRouter(),
			Body:         pack.GetResults(),
			St:           config.TSession_LOGIC,
			ReplyToken:   pack.GetReplyToken(),
			RpcRespCode:  pack.GetRPCRespCode(),
		}

		if a.outChan != nil {
			a.outChan <- replyPack
		}
	}
}

func (a *App) callRPCHandlers(pack IPack) {
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
		if handler, ok := a.rpcRouters[pack.GetRouter()]; ok && handler != nil && len(handler.Funcs) > 0 {

			if handler.NewRoutine {
				// runtine number +1 if block wait
				a.runRoutineChan <- true
				go a.rpcGoRouinteHandler(pack, handler.Funcs)
			} else {
				// range handler rpc
				a.rangeHandler(pack, handler.Funcs)
				//  next results send  link
				a.rpcResultPack(pack)
			}
		} else {
			a.logger.Errorf("App  RPCHandlers NO  router  %s   please add  handler", pack.GetRouter())
		}
	}

}

func (a *App) apiGoRouinteHandler(pack IPack, handlerFuncs []HandlerFunc) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("App receive IPack routine panic ", string(debug.Stack()))
		}
		// runtine number -1
		<-a.runRoutineChan
	}()

	endChan := make(chan bool, 1)
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(a.handleTimeOut)*time.Second)
	go func(ctx context.Context, finishChan chan<- bool, handlerFuncs []HandlerFunc, pack IPack) {
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
		// for range middlerware and handler
		a.rangeHandler(pack, handlerFuncs)
		//  next results send  link
		a.apiResultPack(pack)
	}(ctx, endChan, handlerFuncs, pack)
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

func (a *App) rangeHandler(pack IPack, handlerFuncs []HandlerFunc) {
	//middlerware before
	for _, ware := range a.middlerWares {
		ware.Before(pack)
	}
	//  handler
	for _, handler := range handlerFuncs {
		handler(pack)
		if pack.IsAbort() {
			break
		}
	}
	// middleware after
	for _, ware := range a.middlerWares {
		ware.After(pack)
	}
}

func (a *App) apiResultPack(pack IPack) {
	if len(pack.GetSrcSubRouter()) > 0 && len(pack.GetDstSubRouter()) > 0 {
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
}

func (a *App) callAPIHandlers(pack IPack) {
	if len(pack.GetRouter()) > 0 {
		if handler, ok := a.apiRouters[pack.GetRouter()]; ok && handler != nil && len(handler.Funcs) > 0 {
			if handler.NewRoutine {
				// runtine number +1 if block wait
				a.runRoutineChan <- true
				go a.apiGoRouinteHandler(pack, handler.Funcs)

			} else {
				// for range middlerware and handler
				a.rangeHandler(pack, handler.Funcs)
				//  next results send  link
				a.apiResultPack(pack)
			}
		} else {
			a.logger.Errorf("App API RECALL  router  %s  handler is nil ", pack.GetRouter())
			a.ErrorToConnector(pack.GetSession(), gnError.ErrAPIHandleTimeOutCode, gnError.ErrAPINOHandle.Error())
		}
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

func (a *App) APIRouter(router string, newGoRoutine bool, handlerFunc ...HandlerFunc) {
	if a.apiRouters != nil {
		if handler, ok := a.apiRouters[router]; !ok && handler == nil {
			a.apiRouters[router] = &Handler{
				NewRoutine: newGoRoutine,
				Funcs:      make([]HandlerFunc, 0, 1<<8),
			}
		}
		a.apiRouters[router].Funcs = append(a.apiRouters[router].Funcs, handlerFunc...)
	}
}
func (a *App) RPCRouter(router string, newGoRoutine bool, handlerFunc HandlerFunc) {
	if a.rpcRouters != nil {
		if handler, ok := a.rpcRouters[router]; !ok && handler == nil {
			a.rpcRouters[router] = &Handler{
				NewRoutine: newGoRoutine,
				Funcs:      make([]HandlerFunc, 0, 1<<8),
			}
		}
		a.rpcRouters[router].Funcs = append(a.rpcRouters[router].Funcs, handlerFunc)
	}
}

func (a *App) NewGroup(groupName string) *Group {
	if len(groupName) > 0 {
		var group = NewGroup(a, groupName)
		if a.groups == nil {
			a.groups = new(sync.Map)
		}
		a.groups.Store(groupName, group)
		return group
	}
	return nil

}

func (a *App) GetGroup(groupName string) (*Group, bool) {
	if len(groupName) > 0 && a.groups != nil {
		if group, ok := a.groups.Load(groupName); ok && group != nil {
			if g, ok := group.(*Group); ok && g != nil {
				return g, ok
			}
		}
	}
	return nil, false
}

func (a *App) DelGroup(groupName string) {
	if len(groupName) > 0 && a.groups != nil {
		a.groups.Delete(groupName)
	}
}

func (a *App) AddExceptionHandler(handler gnError.ExceptionHandleFunc) {
	if handler != nil && a.exDetect != nil {
		a.exDetect.AddExceptionHandler(handler)
	}
}

func (a *App) GetLinker() linker.ILinker {
	return a.links
}

func (a *App) UseMiddleWare(middleWare ...GNMiddleWare) {
	a.middlerWares = append(a.middlerWares, middleWare...)
}
