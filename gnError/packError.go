package gnError

import "errors"

const (
	ErrRouter               string = "101"
	ErrPackWrongFormat      string = "102"
	ErrNotBoundHandleCode   string = "103"
	ErrAPIHandleTimeOutCode string = "104"
)

var (
	ErrNotCalculateServerId = errors.New("gn: ErrNotCalculateServerId   please check serverList  ")

	ErrRPCParameter     = errors.New("gn: RPC call serverId  or  router  or  data error ")
	ErrRPCHandleTimeOut = errors.New("gn: rpc message handler timeOut")
	ErrAPIHandleTimeOut = errors.New("gn: api  message handler timeOut")
	ErrRPCNOHandle      = errors.New("gn: rpc handlers no find  this handler ")
	ErrAPINOHandle      = errors.New("gn: api handlers no find  this handler ")
	ErrCHanError        = errors.New("gn: ErrCHanError channel  <-  ! ok ")
	ErrParameter        = errors.New("gn: parameter  error   please check paramter ")

	ErrJsonMarshal           = errors.New("gn: IPackResult message, json JsonMarshal error")
	ErrProtoMarshal          = errors.New("gn: IPackResult message,  protobuf Marshal error")
	ErrAPPCMDMRuning         = errors.New("gn: Appcmd component Runing  error   realdy is runing  ")
	ErrConnectorRuning       = errors.New("gn: Connector Runing  error   realdy is runing  ")
	ErrLinkerRuning          = errors.New("gn: Linker Runing  error   realdy is runing  ")
	ErrMasterRuning          = errors.New("gn: Master Runing  error   realdy is runing  ")
	ErrAPPRuning             = errors.New("gn: APP  Runing  error   realdy is runing  ")
	ErrExceptionDetectRuning = errors.New("gn: Exception component  Runing  error realdy is runing ")
	ErrExceptionDetectChan   = errors.New("gn: Exception component  Channel <-   error ")
	ErrAUnmarshalPbPack      = errors.New("gn: App unmarshal  Pb pack error ")
	ErrCUnmarshalClientPack  = errors.New("gn: connector unmarshal client JSon pack error ")
	ErrCmarshalClientPack    = errors.New("gn: connector marshal client JSon pack error ")
	ErrCmarshalPbPack        = errors.New("gn: connector marshal pb pack error ")
	ErrCUnmarshalPbPack      = errors.New("gn: connector unmarshal pb pack error ")
	ErrNoServerIdPar         = errors.New("gn: start server no serverId paramter")
	ErrConfigLackRoutineNum  = errors.New("gn: config json  no have maxRunRoutineNum par  ")
	ErrMasterCMDMEM          = errors.New("gn: master get remote server Memstats error ")
)

type ExceptionType uint8

const (
	CON_CLOSE ExceptionType = iota
	CON_HARTTMEOUT
	CON_ERROR
	READ_CLOSE
	WRITE_ERROR
	READ_ERROR
	PB_UMARSHAL_ERROR
	NATS_RECONNECT
	NATS_ERRORHANDLER
	NATS__DISCONNECTERR
	NATS_DISCOVERED
	NATS_CLOSED
	NATS_DISCONNECT
	NATS_SENDERROR
	WS_CLOSED
)

type GnException struct {
	Id        string
	Exception ExceptionType
	Msg       string
	Code      int
	BindId    string
}

type PackError struct {
	Code     string `json:"code"`
	ErrorMsg string `json:"errorMsg"`
}
