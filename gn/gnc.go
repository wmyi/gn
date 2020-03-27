package gn

import (
	"github.com/golang/protobuf/proto"
	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
)

// pack
func NewPack(a IApp, ts *config.TSession, session *Session) IPack {
	return &Pack{
		app:     a,
		ts:      ts,
		isAbort: false,
		session: session,
	}
}

// pack
type Pack struct {
	app         IApp
	isAbort     bool
	resultbytes []byte
	session     *Session
	handlerTObj interface{}
	ts          *config.TSession
}

func (p *Pack) Abort() {
	if !p.isAbort {
		p.isAbort = true
	}
}
func (p *Pack) IsAbort() bool {
	return p.isAbort
}
func (p *Pack) GetAPP() IApp {
	return p.app
}
func (p *Pack) GetData() []byte {
	return p.ts.GetBody()
}
func (p *Pack) SetHandlersTranferObj(decodOjb interface{}) {
	if decodOjb != nil {
		p.handlerTObj = decodOjb
	}
}
func (p *Pack) GetHandlersTranferObj() interface{} {
	return p.handlerTObj
}
func (p *Pack) GetSession() *Session {
	return p.session
}
func (p *Pack) GetRouter() string {
	return p.ts.GetRouter()
}
func (p *Pack) ResultJson(obj interface{}) {
	if obj != nil {
		out, err := jsonI.Marshal(obj)
		if err != nil {
			p.app.GetLoger().Errorf("Pack  ResultJson  jsonI.Marshal  err  ", err)
			return
		}
		p.resultbytes = out
	}
}
func (p *Pack) ResultProtoBuf(obj interface{}) {
	if obj != nil {
		pbObj, ok := obj.(proto.Message)
		if !ok {
			p.app.GetLoger().Errorf("Pack  ResultProtoBuf  obj is no proto.Message  type    ")
			return
		}
		out, err := proto.Marshal(pbObj)
		if err != nil {
			p.app.GetLoger().Errorf("Pack  ResultJson  proto.Buffer.Marshal  err     ", err)
			return
		}
		p.resultbytes = out
	}

}
func (p *Pack) ResultBytes(bytes []byte) {
	if len(bytes) > 0 {
		p.resultbytes = bytes
	}
}
func (p *Pack) GetResults() []byte {
	return p.resultbytes
}
func (p *Pack) GetReplyToken() string {
	return p.ts.GetReplyToken()
}
func (p *Pack) GetDstSubRouter() string {
	return p.ts.GetDstSubRouter()
}
func (p *Pack) GetSrcSubRouter() string {
	return p.ts.GetSrcSubRouter()
}
func (p *Pack) GetLogger() *glog.Glogger {
	return p.app.GetLoger()
}
func (p *Pack) GetBindId() string {
	return p.session.GetBindId()
}

// session

func NewSession(cid, scrNodeId, bindId string) *Session {
	return &Session{
		cid:       cid,
		bindId:    bindId,
		srcNodeId: scrNodeId,
	}
}

type Session struct {
	bindId    string
	cid       string
	srcNodeId string
}

func (s *Session) GetCid() string {
	return s.cid
}

func (s *Session) BindId(id string) {
	s.bindId = id
}

func (s *Session) GetBindId() string {
	return s.bindId
}

func (s *Session) GetSrcSubRouter() string {
	return s.srcNodeId
}

// router

type Router struct {
	app        IApp
	apiRouters map[string][]HandlerFunc
	rpcRouters map[string][]HandlerFunc
}

func (r *Router) APIRouter(router string, handlers ...HandlerFunc) {
	if len(router) > 0 {
		r.app.APIRouter(router, handlers...)
	}
}
func (r *Router) RPCRouter(router string, handler HandlerFunc) {
	if len(router) > 0 {
		r.app.RPCRouter(router, handler)
	}
}

// group
type Group struct {
	app         IApp
	groupName   string
	mapSessions map[string]*Session
}

func (g *Group) AddSession(s *Session) {
	if s != nil {
		g.mapSessions[s.GetCid()] = s
	}
}

func (g *Group) BoadCast(bytes []byte) {
	if len(bytes) > 0 && len(g.mapSessions) > 0 {
		for _, value := range g.mapSessions {
			g.app.PushMsg(value, bytes)
		}
	}
}

func NewGroup(app IApp, groupName string) *Group {
	return &Group{
		app:         app,
		groupName:   groupName,
		mapSessions: make(map[string]*Session, 1<<9),
	}
}
