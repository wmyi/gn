package connector

import (
	"bytes"
	"context"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	logger "github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	newline           = []byte{'\n'}
	space             = []byte{' '}
	cidAddBase uint64 = 10000
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024
)

type WSConnection struct {
	RChan chan []byte
	WChan chan *ChanMsgPack
	// The websocket connection.
	wsConn          *websocket.Conn
	cid             string
	bindId          string
	readRoutineCan  context.CancelFunc
	writeRoutineCan context.CancelFunc
	isRuning        bool
	exDetect        *gnError.GnExceptionDetect
}

func (wc *WSConnection) SendMessage(data []byte) {
	if wc.RChan != nil {
		wc.RChan <- data
	}
}

func (wc *WSConnection) GetConnectionCid() string {
	return wc.cid
}

// read Message from WS
func (wc *WSConnection) ReadMsg(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("ws  soeckt Read Routine panic ", string(debug.Stack()))
			// wc.wsConn.Close()
		}
	}()
	wc.wsConn.SetReadLimit(maxMessageSize)
	wc.wsConn.SetReadDeadline(time.Now().Add(pongWait))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}
		msgType, message, err := wc.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("ws  soeckt Read err- ", err)
			}
			logger.Errorf("ws  soeckt Read err-   %v", err)
			break
		}
		if msgType == websocket.TextMessage {
			message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		}
		if wc.WChan != nil {
			wc.WChan <- &ChanMsgPack{
				cid:         wc.cid,
				body:        message,
				logicBindID: wc.GetBindId(),
			}
		}
		logger.Infof("ws  soeckt receive  id:  %d  msg  %s ", wc.bindId, string(message))
	}
}

func (wc *WSConnection) Done() {
	if wc.isRuning {
		wc.wsConn.Close()

		if wc.writeRoutineCan != nil {
			wc.writeRoutineCan()
		}
		if wc.readRoutineCan != nil {
			wc.readRoutineCan()
		}
		wc.isRuning = false
	}
}

// send Message  to WS
func (wc *WSConnection) WriteMsg(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			ticker.Stop()
			wc.wsConn.Close()
			logger.Errorf("ws  soeckt write Routine panic ", string(debug.Stack()))
		}
	}()

	for {
		select {
		case message, ok := <-wc.RChan:
			logger.Infof("ws  write msg   %v ", string(message))
			wc.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// if !ok {
				// The input closed the channel.
				wc.wsConn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := wc.wsConn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			// Add queued chat messages to the current websocket message.
			n := len(wc.RChan)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-wc.RChan)
			}
			if err := w.Close(); err != nil {
				logger.Errorf("ws  write msg close err   %v ", err)
				return
			}
		case <-ticker.C:
			wc.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := wc.wsConn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Errorf("ws  write msg timeout     ")
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (wc *WSConnection) CloseHandler(code int, text string) error {
	wc.exDetect.ThrowException(&gnError.GnException{
		Id:        wc.cid,
		Exception: gnError.WS_CLOSED,
		Msg:       text,
		Code:      code,
		BindId:    wc.bindId,
	})
	return nil
}

func (wc *WSConnection) Run() {
	if !wc.isRuning {
		// read routine
		rctx, cancal := context.WithCancel(context.Background())
		go wc.ReadMsg(rctx)
		wc.readRoutineCan = cancal
		wctx, cancal := context.WithCancel(context.Background())
		// write routine
		go wc.WriteMsg(wctx)
		wc.writeRoutineCan = cancal
		wc.isRuning = true

		if wc.wsConn != nil {
			wc.wsConn.SetCloseHandler(wc.CloseHandler)
			wc.wsConn.SetPongHandler(func(string) error { wc.wsConn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
		}

	}
}

func (wc *WSConnection) GetConnection() interface{} {
	return wc.wsConn
}

func (wc *WSConnection) GetBindId() string {
	return wc.bindId
}

func (wc *WSConnection) SetBindId(bindId string) {
	wc.bindId = bindId
}

func NewWSConnection(conn *websocket.Conn, outChan chan *ChanMsgPack,
	detect *gnError.GnExceptionDetect) *WSConnection {
	return &WSConnection{
		wsConn:   conn,
		RChan:    make(chan []byte, 10),
		WChan:    outChan,
		cid:      uuid.New().String() + "-" + serverId + "-" + strconv.FormatUint(atomic.AddUint64(&cidAddBase, 1), 10),
		isRuning: false,
		exDetect: detect,
	}
}
