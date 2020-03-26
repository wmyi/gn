package linker

import (
	"strconv"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"

	"github.com/nats-io/nats.go"
)

type natsLinker struct {
	natsC        *nats.Conn
	CWChan       chan []byte
	natsConf     *config.Natsconfig
	serverID     string
	logger       *glog.Glogger
	natsClientId uint64
	isRuning     bool
	exDetect     *gnError.GnExceptionDetect
}

func NewNatsClient(serverId string, natsConf *config.Natsconfig, outChan chan []byte, log *glog.Glogger,
	detect *gnError.GnExceptionDetect) *natsLinker {
	nats := &natsLinker{
		CWChan:   outChan,
		natsConf: natsConf,
		serverID: serverId,
		logger:   log,
		isRuning: false,
		exDetect: detect,
	}
	return nats
}

func (nl *natsLinker) Done() {
	if nl.natsC.IsConnected() {
		defer nl.natsC.Close()
		defer nl.natsC.Flush()
	}
	if nl.isRuning {
		nl.isRuning = false
	}
}

func (nl *natsLinker) natsDisconnectHandler(con *nats.Conn) {
	nl.exDetect.ThrowException(&gnError.GnException{
		Id:        nl.serverID,
		Exception: gnError.NATS_DISCONNECT,
		Msg:       "Nats connection  DisconnectErrException will set the disconnect event handler.",
	})
}

func (nl *natsLinker) natsClosedHandler(con *nats.Conn) {
	nl.exDetect.ThrowException(&gnError.GnException{
		Id:        nl.serverID,
		Exception: gnError.NATS_CLOSED,
		Msg:       "Nats connection ClosedException will set the reconnect event handler.",
	})
}

func (nl *natsLinker) natsDiscoveredServersHandler(con *nats.Conn) {
	nl.exDetect.ThrowException(&gnError.GnException{
		Id:        nl.serverID,
		Exception: gnError.NATS_DISCOVERED,
		Msg:       "Nats connection DiscoveredServersException will set the discovered servers handler.",
	})
}
func (nl *natsLinker) natsDisconnectErrHandler(con *nats.Conn, err error) {
	nl.exDetect.ThrowException(&gnError.GnException{
		Id:        nl.serverID,
		Exception: gnError.NATS__DISCONNECTERR,
		Msg:       "Nats connection  DisconnectErrException will set the disconnect event handler.",
	})
}

func (nl *natsLinker) natsErrorHandler(*nats.Conn, *nats.Subscription, error) {
	nl.exDetect.ThrowException(&gnError.GnException{
		Id:        nl.serverID,
		Exception: gnError.NATS_ERRORHANDLER,
		Msg:       "Nats connection ErrorException will set the async error handler.",
	})
}
func (nl *natsLinker) natsReconnectHandler(con *nats.Conn) {

	nl.exDetect.ThrowException(&gnError.GnException{
		Id:        nl.serverID,
		Exception: gnError.NATS_RECONNECT,
		Msg:       "Nats connection ReconnectException will set the reconnect event handler.",
	})
}

func (nl *natsLinker) GetConnection() interface{} {
	return nl.natsC
}
func (nl *natsLinker) Run() error {
	if nl.natsC == nil && !nl.isRuning {
		var address string = nl.natsConf.Host + ":" + strconv.Itoa(nl.natsConf.Port)
		connect, err := nats.Connect(address)

		if err != nil {
			nl.logger.Errorf("nats  connect   ", err)
			return err
		}
		nl.natsC = connect
		// set handler
		nl.natsC.SetClosedHandler(nl.natsClosedHandler)
		nl.natsC.SetDisconnectErrHandler(nl.natsDisconnectErrHandler)
		nl.natsC.SetDisconnectHandler(nl.natsDisconnectHandler)
		nl.natsC.SetDiscoveredServersHandler(nl.natsDiscoveredServersHandler)
		nl.natsC.SetErrorHandler(nl.natsErrorHandler)
		nl.natsC.SetReconnectHandler(nl.natsReconnectHandler)

		err = nl.initNats()
		if err != nil {
			return err
		}
		nl.isRuning = true
		return nil
	} else {
		return gnError.ErrLinkerRuning
	}
}

func (nl *natsLinker) SendMsg(router string, data []byte) {
	if nl.natsC != nil {
		err := nl.natsC.Publish(router, data)
		if err != nil {
			nl.logger.Errorf("natsLinker sendMsg   error  ", err)
			nl.exDetect.ThrowException(&gnError.GnException{
				Id:        nl.serverID,
				Exception: gnError.NATS_SENDERROR,
				Msg:       err.Error(),
			})
		}
	}

}

func (nl *natsLinker) GetSubRounter() string {
	return nl.serverID
}

func (nl *natsLinker) initNats() error {
	if nl.natsC != nil {

		_, err := nl.natsC.Subscribe(nl.GetSubRounter(), func(msg *nats.Msg) {
			nl.logger.Infof("nats  receive   msg  %s   \n", string(msg.Data))
			nl.CWChan <- msg.Data
		})
		if err != nil {
			nl.Done()
			nl.logger.Errorf("nats subscribe error  %v", err)
			return err
		}

		nl.natsClientId, err = nl.natsC.GetClientID()
		if err == nil {
			nl.logger.Infof("natsLinker Run done  urls: %v clientId: %d Subscribe: %s  ", nl.natsC.Servers(), nl.natsClientId, nl.GetSubRounter())
		} else {
			return err
		}

	}
	return nil
}
