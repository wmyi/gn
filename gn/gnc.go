package gn

import (
	"sync"

	"github.com/wmyi/gn/config"
	"github.com/wmyi/gn/glog"
	"github.com/wmyi/gn/gnError"
	"github.com/wmyi/gn/gnutil"
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
	out, ok := gnutil.JsonToBytes(obj, p.app.GetLogger())
	if ok && out != nil {
		p.resultbytes = out
	}
}

func (p *Pack) ExceptionAbortJson(code, msg string) {
	if len(code) > 0 && len(msg) > 0 {
		p.ResultJson(gnError.PackError{
			Code:     code,
			ErrorMsg: msg,
		})
		p.Abort()
	}
}

func (p *Pack) ResultProtoBuf(obj interface{}) {

	out, ok := gnutil.ProtoBufToBytes(obj, p.app.GetLogger())
	if ok && out != nil {
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
	return p.app.GetLogger()
}
func (p *Pack) GetBindId() string {
	return p.session.GetBindId()
}

func (p *Pack) SetRPCRespCode(code int) {
	p.ts.RpcRespCode = int32(code)
}
func (p *Pack) GetRPCRespCode() int32 {
	return p.ts.RpcRespCode
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
	mapSessions *sync.Map
}

func (g *Group) AddSession(key string, s *Session) {
	if s != nil && len(key) > 0 {
		g.mapSessions.Store(key, s)
	}
}

func (g *Group) DelSession(key string) {
	if len(key) > 0 {
		g.mapSessions.Delete(key)
	}
}

func (g *Group) GetSession(key string) (*Session, bool) {
	if len(key) > 0 {
		if s, ok := g.mapSessions.Load(key); ok {
			if ss, ok := s.(*Session); ok {
				return ss, ok
			}
		}
	}
	return nil, false
}

func (g *Group) BroadCast(bytes []byte) {

	if len(bytes) > 0 {
		g.mapSessions.Range(func(key, value interface{}) bool {
			if s, ok := value.(*Session); ok && s != nil {
				g.app.PushMsg(s, bytes)
			}
			return true
		})
	}
}

func (g *Group) BroadCastJson(obj interface{}) {
	out, ok := gnutil.JsonToBytes(obj, g.app.GetLogger())
	if ok && out != nil {
		g.BroadCast(out)
	}
}

func (g *Group) BroadCastProtoBuf(obj interface{}) {
	out, ok := gnutil.ProtoBufToBytes(obj, g.app.GetLogger())
	if ok && out != nil {
		g.BroadCast(out)
	}

}

func NewGroup(app IApp, groupName string) *Group {
	return &Group{
		app:         app,
		groupName:   groupName,
		mapSessions: new(sync.Map),
	}
}
