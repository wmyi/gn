package connector

import (
	"context"
	"flag"
	"hash/crc32"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/linker"

	"github.com/wmyi/gn/gn"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

var (
	instance *Connector
	once     sync.Once
	jsonI    = jsoniter.ConfigCompatibleWithStandardLibrary
	serverId string
	mode     string
)

type RouteRearEndHFunc func(cid, bindId string, serverList []*config.ServersConfig) (serverId string)
type CheckOriginHFunc func(r *http.Request) bool
type VerifyClientConnectHFunc func(r *http.Request) bool

type Connector struct {
	cms          *sync.Map
	CRChan       chan *ChanMsgPack
	CWChan       chan []byte
	CRRoutineCan context.CancelFunc
	CWRoutineCan context.CancelFunc
	LinkerClient linker.ILinker
	config       *config.Config

	rearEndHandles    map[string]RouteRearEndHFunc
	logger            *glog.Glogger
	cmdMaster         gn.IAppCmd
	exDetect          *gnError.GnExceptionDetect
	checkOriginHand   CheckOriginHFunc
	verifyConnectHand VerifyClientConnectHFunc
	isRuning          bool
	wsUpgrader        *websocket.Upgrader
}

func (c *Connector) Done() {
	if c.isRuning {
		c.isRuning = false
		if c.CRRoutineCan != nil {
			c.CRRoutineCan()
		}
		if c.CWRoutineCan != nil {
			c.CWRoutineCan()
		}
		if c.LinkerClient != nil {
			c.LinkerClient.Done()
		}

		if c.cmdMaster != nil {
			c.cmdMaster.Done()
		}
		if c.exDetect != nil {
			c.exDetect.Done()
		}
		if c.logger != nil {
			c.Done()
		}
		c.cms.Range(func(key, value interface{}) bool {
			if ws, ok := value.(WSConnection); ok {
				ws.Done()
			}
			c.cms.Delete(key)
			return true
		})
		c.isRuning = false
	}
}

func (c *Connector) SetCheckOriginHandler(handler CheckOriginHFunc) {
	if handler != nil {
		c.checkOriginHand = handler
	}
}
func (c *Connector) SetVerifyConnectHandler(handler VerifyClientConnectHFunc) {
	if handler != nil {
		c.verifyConnectHand = handler
	}
}

func (c *Connector) SendPack(serverAddress, router, bindId, cid string, data []byte) {
	if len(serverAddress) > 0 && len(router) > 0 && len(bindId) > 0 {
		pack := &config.TSession{
			Cid:          cid,
			SrcSubRouter: c.LinkerClient.GetSubRounter(),
			DstSubRouter: serverAddress,
			Body:         data,
			St:           config.TSession_CONNECTOR,
			Router:       router,
			LogicBindId:  bindId,
		}
		out, err := proto.Marshal(pack)
		if err == nil {
			c.logger.Infof("connector  SendPack  serverAddress   %v   Msg  %v  ", serverAddress, out)
			c.LinkerClient.SendMsg(serverAddress, out)
		} else {
			c.logger.Errorf("connector  SendPack Marshal Pb  errr   %v  ", err)
		}
	}

}

func (c *Connector) decodeClientPack(pack *ChanMsgPack) error {

	c.logger.Infof("connector- client Pack bindId  %s   receive   %v  ", pack.logicBindID, string(pack.body))
	var mapData map[string]interface{}
	// unmarshal  json
	err := jsonI.Unmarshal(pack.body, &mapData)
	if err != nil {
		c.logger.Errorf("connector receive  Msg   Unmarshal  err   %v  ", err)
		// return  to client
		c.ErrorToClient(pack.cid, gnError.ErrPackWrongFormat, "pack format wrong json  please check ")
		return gnError.ErrCUnmarshalClientPack
	}
	if routerS, ok := mapData["router"]; ok {
		if router, ok := routerS.(string); ok {
			routers := strings.Split(router, ".")
			c.logger.Infof("connector-   router    %v \n ", routers)
			if len(routers) > 1 {

				// calculate end serverAddress
				serverAddress := c.GetServerIdByRouter(routers[0], pack.logicBindID, pack.cid, c.config.GetServerByType(routers[0]))
				// marshal msg to  nats   or  end server
				if len(serverAddress) > 0 && len(routers[1]) > 0 {
					// push
					pack := &config.TSession{
						Cid:          pack.cid,
						SrcSubRouter: c.LinkerClient.GetSubRounter(),
						DstSubRouter: serverAddress,
						Body:         pack.body,
						St:           config.TSession_CONNECTOR,
						Router:       routers[1],
						LogicBindId:  pack.logicBindID,
					}
					out, err := proto.Marshal(pack)
					if err == nil {
						c.logger.Infof("connector  send  serverAddress   %v   Msg  %v  ", serverAddress, out)
						c.LinkerClient.SendMsg(serverAddress, out)
					} else {
						c.logger.Errorf("connector  send Marshal Pb  errr   %v  ", err)
						return gnError.ErrCmarshalPbPack
					}
				} else {
					c.ErrorToClient(pack.cid, gnError.ErrRouter, "please check router  format")
					c.logger.Infof("connector- recve cid  %s  msg  %v   route fail  please check route \n", pack.cid, string(pack.body))
				}
			} else {
				c.logger.Infof("recve  router  error      %v   ", router)
				c.ErrorToClient(pack.cid, gnError.ErrRouter, "please check router  format")
			}
		} else {
			c.ErrorToClient(pack.cid, gnError.ErrRouter, "please check router  format")
		}
	} else {
		c.ErrorToClient(pack.cid, gnError.ErrRouter, "please check router  format")
	}

	return nil
}

func (c *Connector) LoopClientReadChan(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("connector Read Routine panic ", string(debug.Stack()))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-c.CRChan:
			if ok {
				c.decodeClientPack(data)
			}
		}
	}
}

func (c *Connector) ErrorToClient(cid string, code string, errorMsg string) {

	packResponse := &gnError.PackError{
		Code:     code,
		ErrorMsg: errorMsg,
	}
	if c.CRChan != nil {
		out, err := jsonI.Marshal(packResponse)
		if err != nil {
			c.logger.Errorf("connector ErrorToClient  jsonI.Marshal  err  ", err)
			return
		}

		pack := &config.TSession{
			Cid:  cid,
			Body: out,
		}
		packOut, err := proto.Marshal(pack)
		if err != nil {
			c.logger.Errorf("connector ErrorToClient  proto.Marshal  err  ", err)
			return
		}
		c.CWChan <- packOut
	}
}

func (c *Connector) GetServerIdByRouter(serverType string, LogicBindId string, cid string, serverList []*config.ServersConfig) string {
	if len(LogicBindId) > 0 && len(c.rearEndHandles) > 0 {
		handler := c.rearEndHandles[serverType]
		if handler != nil {
			serverId := handler(cid, LogicBindId, serverList)
			return serverId
		}
	}
	index := int(crc32.ChecksumIEEE([]byte(cid))) % len(serverList)
	if serverList[index] != nil {
		return serverList[index].ID
	}
	return ""
}

func (c *Connector) AddRouterRearEndHandler(serverType string, handler RouteRearEndHFunc) {
	if c.rearEndHandles != nil && len(serverType) > 0 && handler != nil {
		c.rearEndHandles[serverType] = handler
	}
}

func (c *Connector) decodeRearEndPbPack(data []byte) error {
	if len(data) > 0 {
		dTsession := &config.TSession{}
		c.logger.Infof(" connector  RearEnd receive msg      %v ", string(data))
		err := proto.Unmarshal(data, dTsession)
		if err != nil {
			c.logger.Errorf("connector  RearEnd msg  UnMarshal err   %v ", err)
			return gnError.ErrCUnmarshalPbPack
		}

		if dTsession.GetSt() == config.TSession_CMD {
			if c.cmdMaster != nil {
				pack := gn.NewPack(nil, dTsession, gn.NewSession(dTsession.GetCid(), dTsession.GetSrcSubRouter(), dTsession.GetLogicBindId()))
				c.cmdMaster.ReceiveCmdPack(pack)
			}
		} else {
			cid := dTsession.GetCid()
			if wsI, ok := c.cms.Load(cid); ok {
				if ws, ok := wsI.(IConnection); ok && ws != nil {
					// bind   logic Id
					if len(ws.GetBindId()) == 0 {
						ws.SetBindId(dTsession.GetLogicBindId())
					}
					ws.SendMessage(dTsession.GetBody())
				}
			}
		}
	}
	return nil

}

func (c *Connector) GetLinker() linker.ILinker {
	return c.LinkerClient
}

func (c *Connector) LoopLinkerChan(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("connector receive Nats routine panic ", string(debug.Stack()))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-c.CWChan:
			if ok {
				c.decodeRearEndPbPack(data)
			}
		}

	}

}

func (c *Connector) ListenAndRun() error {
	// websocket   init
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// verifyClientConnect
		if c.verifyConnectHand != nil && !c.verifyConnectHand(r) {
			c.logger.Infof(" ws  connector verify fail  url  %v", r.RequestURI)
			return
		}
		// init wsUpgrader
		if c.wsUpgrader == nil {
			c.wsUpgrader = &websocket.Upgrader{
				// 允许所有CORS跨域请求
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			}
			if c.checkOriginHand != nil {
				c.wsUpgrader.CheckOrigin = c.checkOriginHand
			}
		}
		wsConn, err := c.wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			c.logger.Errorln(err)

			return
		}
		// create connection
		client := NewWSConnection(wsConn, c.CRChan, c.logger, c.exDetect)
		uuid := client.GetConnectionCid()
		if len(uuid) > 0 {
			c.cms.Store(uuid, client)
			client.Run()
			c.logger.Infof("connector new  connection uuid    %s ", uuid)
		} else {
			c.logger.Errorln("connector   connection uuid     %s ")
		}
	})
	var host string = c.config.GetConConfByServerId(serverId).Host + ":" + strconv.Itoa(c.config.GetConConfByServerId(serverId).ClientPort)
	if err := http.ListenAndServe(host, nil); err != nil {
		c.logger.Infof("http ListenAndServe  error %v", err)
		return err
	}
	return nil
}

func (c *Connector) ExceptionHandler(exception *gnError.GnException) {
	if exception.Exception == gnError.WS_CLOSED {
		if ws, ok := c.cms.Load(exception.Id); ok && ws != nil {
			wsCon, ok := ws.(*WSConnection)
			if ok {
				wsCon.Done()
			}
			c.cms.Delete(exception.Id)
		}
	}
}

func (c *Connector) Run() error {

	if !c.isRuning {
		// nats  connection
		err := c.LinkerClient.Run()
		if err != nil {
			c.logger.Errorf("natsLinker Run error  %v", err)
			return err
		}
		err = c.cmdMaster.Run()
		if err != nil {
			c.logger.Errorf("cmdApp Run error  %v", err)
			return err
		}
		rctx, rcancal := context.WithCancel(context.Background())
		go c.LoopClientReadChan(rctx)
		c.CRRoutineCan = rcancal
		wctx, wcancal := context.WithCancel(context.Background())
		go c.LoopLinkerChan(wctx)
		c.CWRoutineCan = wcancal
		c.exDetect.Run(true)
		c.isRuning = true
		// set exception
		c.exDetect.AddExceptionHandler(c.ExceptionHandler)

		// listen http
		return c.ListenAndRun()
	} else {
		c.logger.Errorf("connector is aleady Runing ")
		return gnError.ErrConnectorRuning
	}

}

func (c *Connector) AddExceptionHandler(handler gnError.ExceptionHandleFunc) {
	if handler != nil && c.exDetect != nil {
		c.exDetect.AddExceptionHandler(handler)
	}
}

func ParseCommands() {
	commands := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	commands.StringVar(&serverId, "serverId", "", " server id   logic server  who  ")
	commands.StringVar(&mode, "mode", "", " mode is debug  log is std.out  defalut is logs file ")
	commands.Parse(os.Args[1:])
}

func DefaultConnector(config *config.Config) (IConnector, error) {
	ParseCommands()
	if len(serverId) <= 0 {
		return nil, gnError.ErrNoServerIdPar
	}
	once.Do(func() {
		instance = &Connector{
			cms:            &sync.Map{},
			CRChan:         make(chan *ChanMsgPack, 1024),
			CWChan:         make(chan []byte, 1024),
			config:         config,
			rearEndHandles: make(map[string]RouteRearEndHFunc, 8),
			isRuning:       false,
		}
		instance.logger = glog.NewLogger(config, serverId, mode)
		instance.exDetect = gnError.NewGnExceptionDetect(instance.logger)

		instance.LinkerClient = linker.NewNatsClient(config.GetConConfByServerId(serverId).ID, &config.Natsconf,
			instance.CWChan, instance.logger, instance.exDetect)
		instance.cmdMaster = gn.NewAppCmd(instance.logger, instance.LinkerClient, instance.exDetect)
	})
	return instance, nil
}

func (c *Connector) GetLoger() *glog.Glogger {
	return c.logger
}
