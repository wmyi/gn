# GN
###### GN是由golang开发的一个 轻量级，上手简单的游戏服务器框架。  <br>
###### GN的RPC调用采用了消息中间件nats，后期可能会扩展纯RPC框架 如GRPC或者gnet等 目前还在酝酿中。<br>
###### GN 的名字正是golang和nats中间件的首字母而来 <br>


--------------
# GN 框架图
![GNFramework](./doc/img/GNFramework.jpg "GNFramework")


--------------

# GN 简介
## 1：GN 服务简介
GN 游戏框架，目前主要分为 三种 游戏服务器类型  connector服务类型，APP服务类型，master服务类型 ，另外包含一中间件nats 服务。<br>


* `connector服务`:连接器服务-》可以理解为 客户端连接后端服务器集群的前端网关服务，主要做客户端socket通道管理，消息的路由 以及中转消息包等相关业务逻辑，不涉及具体的业务逻辑开发，目前支持websocket 连接，后续会扩展其他连接。该服务通常会根据业务需求，扩展该服务节点数。

* `APP服务`:游戏主服务-》 游戏业务的主服务，几乎所有的业务逻辑以及开发接口，都需要此对象注册，一个前端或RPC 消息包Ipack的流程处理，中间件，group 等等，也都由APP 对象管理。通常该服务类型，有多个 APP服务，来组成游戏集群，供其他服务调用。而config配置文件的servers->"id":"login-001",来确定该服务的唯一性，serverType,来确定 该APP服务类型，多个相同serverType，组成一组相同功能的游戏服务。其他服务根据路由算法确定 调用其中一个游戏服务。


* `master服务`:master服务 正如其名 其他游戏服务的master节点，主要用来 ping/pong 其他服务节点信息来检测 以及获取其他服务节点自动数据返回，如查看内存活跃用户，等等，但是需要用户自己注册handler 返回自定义数据，该服务 不是必要条件，可以根据需求 判断是否启动该服务。该服务  通常为一个节点 获取所连接子节点的相应API 调用。


* `nats 服务`:nats是一个高性能，易使用，轻量级 golang开发的 中间件服务，GN主要用来做 消息中转服务，而不需要每个服务节点之间 都连接socket。nats默认
不支持 消息落盘服务，这样也会影响性能，用户可以根据 自定义 自己选择是否需要落盘

## 2: handler API & RPCAPI
GN可以根据自己后端API需求，轻松简单实现 该API是 单独协程处理业务逻辑，还是公用一个协程排队处理，如：<br>
```go
APIRouter(router string, newGoRoutine bool, handlerFunc ...HandlerFunc)
RPCRouter(router string, newGoRoutine bool, handlerFunc HandlerFunc)
```
使用样例：
```go
app.APIRouter("addGroup", true, addGroup)
app.APIRouter("createGroup", true, createGroup)
app.RPCRouter("rpcGetAllGroups", false, rpcGetAllGroups)
```

## 3: GN 名词 及 重要component 组件简介 
* `IPack组件`: APP游戏服务-》 通讯消息包的封装或者路由的上下文的封装，该消息包从收到到最终逻辑处理完毕，的整个流程的上下文。

* `Group组件`: App游戏服务-》group组件 正如其名，可以把他看成一个组，比如一个聊天室，或者房间都可以做一个组，该group主要用来存储 connector 服务的session等信息，该组件支持群发消息，添加用户session，删除session，等相关方法。该group KEY-》group 方式 存储在APP  对象的内存中，重启会丢失。

* `GNMiddleWare组件` :APP游戏服-》中间件，包含Before(IPack)，After(IPack) 两个方法，主要是用来全局，处理Ipack消息包的前置拦截或者后置的消息的处理 操作。

* `config组件`: APP游戏服务/connector服务/master服务-》游戏集群服务的配置文件，nats 配置文件，connector，APP，等相关服务器的配置 ，服务启动需要设置路径，并根据config模块 启动各个服务

* `Viper组件`: APP游戏服务-》业务逻辑相关 配置文件，该github-》 viper 支持json，yaml等格式的配置文件，viper 配置可以在APP服务启动前设置，比如：
    app.AddConfigFile("test", "../../config/", "yaml")。  

* `Session组件`:APP游戏服务-》session 主要为后端业务逻辑服务，用来绑定前端connector服务的连接ID和 用户逻辑ID绑定而出现的。session：包含 1）连接服的websocket 连接ID，2）用户BindID，3）所在前端服务器的唯一serverID。该session 可用group组件 统一保存在内存，并维护。用户断线重连 该session和老session可能会重复，需要用户自己处理

* `glog Log组件`: APP游戏服务/connector服务/master服务-》 该log 模块 是根据 golang 官方log包，修改以及包装而来，log模块相关配置可以直接在 config配置文件配置，如果不配置，默认为 终端重开输出，不会输出为文件，该log模块 默认一个协程 处理log 的写入。避免因为写入log影响其他模块的性能。

* `CMDHandler方法`: APP游戏服务/connector服务-》该方法主要是用来注册cmdAPI  func (a *App) CMDHandler(cmd string, handler HandlerFunc)  主要 用来 接收master  服务的cmd 命令，然后根据需求，返回 master方法，该方法master 对应的方法：SendCMD(cmd,nodeId,string data []byte),cmd为注册的唯一 API名字，需要唯一，master的SendCMD为协程阻塞方法。

* `ILinker 连接器组件`:APP游戏服务/connector服务/master服务-》ILinker 连接器，为 各个 服务节点 连接nats中间件的   封装，主要用发送和接收消息包。
该 ILinker 接口实现为 natsLinker，后续会有其他 连接器如GRPC Linker

* `WSConnection 客户端连接器组件`:connector服务-》 客户端连接 服务器集群connector 服务的 ，封装，主要用来接收，和发送 消息包到 客户端！ 该实现 为github.com/gorilla/websocket 的封装

* `GnExceptionDetect 异常处理组件`:APP游戏服务/connector服务/master服务-》该组件 用来 注册 各个服务异常的回调的函数注册，样例如下：
```go 
	// exception handler
	connector.AddExceptionHandler(func(exception *gnError.GnException) {
		// close handler push msg
		if exception.Exception == gnError.WS_CLOSED && len(exception.BindId) > 0 && len(exception.Id) > 0 {
			handlerName := "wsclose"
			serverAddress := connector.GetServerIdByRouter(handlerName, exception.BindId, exception.Id,
				config.GetServerByType("login"))
			connector.SendPack(serverAddress, handlerName, exception.BindId, exception.Id, nil)
		}
	})
```
    所有exception异常 请在gnError包查看


------------
## 4:GN master 服务 样例及说明
#### 目前GN master 模块 并没有 增加 启动，重启以及管理的 子节点的功能，而是简单的添加了  探知各个子节点，ping/pong心跳机制，以及master RPC 远程调用  其他子节点的注册函数，返回相应的 信息，以供开发者 获取 子节点对应的 子服务的信息。 例子如下：<br>
``` go 
	// master 实例代码  master 调用   login-001 服务商的 online 方法
	// routine  master  不要阻塞主协程
	go func() {
		for {
			time.Sleep(3 * time.Second)
			req := message.CmdOnlineReq{
				Admin: "test",
				Msg:   "test test test",
			}
								// 参数说明 cmd： API名字 ，需要唯一性， 明确指出 RPC  调用的APIname
								// nodeId ：服务唯一ID，具体参考 配置文件
								// obj  : RPC  远程调用的参数 
			results, err := master.SendCMDJson("online", "login-001", req)
			if err != nil {
				logger.Infof("master.SendCMD online error  %v   \n ", err)
				return
			}
			if len(results) > 0 {
				res := &message.CmdOnlineRes{}
				jsonI.Unmarshal(results, res)
				logger.Infof("master.SendCMD -- online  results  %v ", res)
			}

		}
	}()

	// 节点 login-001 实现  online  方法 实例
	// master cmd  command
	app.CMDHandler("online", func(pack gn.IPack) {
		req := &message.CmdOnlineReq{}
		logger.Infof("online Cmd--    %v \n", string(pack.GetData()))
		if err = jsonI.Unmarshal(pack.GetData(), req); err == nil {
			logger.Infof("  online    req    %v \n ", req)
			if req.Admin == "test" {
				response := &message.CmdOnlineRes{
					ServerId: "login-001",
					Msg:      "onLine test  test",
				}
				logger.Infof("  online    response    %v \n ", response)
				pack.ResultJson(response)
			}
		}
	})

```


------------
## 5: GN APP 服务 样例及说明
#### GN APP  是整个游戏框架的核心，所有的业务逻辑基本都在此APP，也是开发者接触最多的地方。 包括注册API，RPC的API，以及中间件等等
``` go
	//1 app 创建 例子
	config, err := config.NewConfig("../../config/development.json")
	if err != nil {
		logger.Infof("config  error\n ", err)
		return
	}
	app, err := gn.DefaultApp(config) // 默认创建APP，项目
	if err != nil {
		logger.Infof("new APP error %v \n", err)
		return
	}
	err = app.Run()
	if err != nil {
		logger.Infof("loginApp run   error   %v \n", err)
		return
	}
	defer app.Done()
	// config 配置json 会在后续说明 或者参考 chat项目

	//2 注册API  例子
	func InitAPIRouter(app gn.IApp) {
		app.APIRouter("login", true, Login)
		app.APIRouter("logout", true, Logout)
		app.APIRouter("chat", false, Chat)
		app.APIRouter("wsclose", true, WsClosedHandler)
	}
	func InitRPCRouter(app gn.IApp) {
		app.RPCRouter("rpcGetAllUsers", false, rpcGetAllUsers)
		app.RPCRouter("notifyCreateGroup", false, notifyCreateGroup)
	}
	// 根据自己的 需求  是否单独分开文件  参数简介 ：API  名字，是否开启新协程，调用此方法，函数

	// 3 添加 逻辑配置文件 样例
	// 具体viper 使用可以参考  "github.com/spf13/viper"  开源项目
	app.AddConfigFile("test", "../../config/", "yaml")
	//4 添加 中间件 文件 样例
	app.UseMiddleWare(&middlerware.PackTimer{})
	// 中间件样例 接口
	type GNMiddleWare interface {
		Before(IPack)
		After(IPack)
	}
	// 实现样例
	type PackTimer struct {
	}

	func (t *PackTimer) Before(pack gn.IPack) {
		reqId := rand.Intn(1 << 10)
		nowTime := time.Now()
		pack.SetContextValue("reqId", reqId)
		pack.SetContextValue("inTime", nowTime)
		logger.Infof("Before reqId: %d    time %v  ", reqId, nowTime)
	}

	func (t *PackTimer) After(pack gn.IPack) {
		reqId := pack.GetContextValue("reqId").(int)
		nowTime := pack.GetContextValue("inTime").(time.Time)
		logger.Infof("After reqId: %d   diff time %v  ", reqId, time.Now().Sub(nowTime))
	}
	//5 API 注册 接口样例 & RPC接口 样例
	// 聊天
	// API  
	app.APIRouter("chat", false, Chat)


	func Chat(pack gn.IPack) {

		logger.Infof("loginApp  Chat   pack  data %v \n", string(pack.GetData()))

		//unmarshal json
		reqData := &message.LoginReq{}
		// request  data
		if len(pack.GetData()) > 0 {
			err := jsonI.Unmarshal(pack.GetData(), reqData)
			if err != nil {
				pack.ExceptionAbortJson("101", "解析前端数据失败 JSON ")
				return
			}
		}
		// logic
		// response to connector
		app := pack.GetAPP()
		if len(reqData.Bridge) > 0 {
			group, ok := app.GetGroup("userSession")
			if ok && group != nil {
				for _, uid := range reqData.Bridge {
					s, ok := group.GetSession(uid)
					if ok && s != nil {
						respon := &message.ChatRes{
							Router:   "chat",
							Date:     time.Now().Format("2006-01-02 15:04:05"),
							Msg:      reqData.Msg,
							Nickname: reqData.Nickname,
							UID:      reqData.UID,
							Bridge:   reqData.Bridge,
							GroupID:  reqData.GroupID,
							Status:   1,
						}
						// push other user msg
						app.PushJsonMsg(s, respon)
					}
				}
			}
		}
	}
	// RPC 
	app.RPCRouter("rpcGetAllUsers", false, rpcGetAllUsers)
	func rpcGetAllUsers(pack gn.IPack) {
		// get all groups return

		users, ok := pack.GetAPP().GetObjectByTag("userList")
		if ok && users != nil {
			if mUsers, ok := users.(map[string]*model.UserMode); ok {
				userList := make([]interface{}, 0, len(mUsers))
				for _, item := range mUsers {
					userList = append(userList, item)
				}
				pack.ResultJson(userList)
				pack.SetRPCRespCode(0)
			} else {
				pack.SetRPCRespCode(102)
			}

			return
		}

		pack.SetRPCRespCode(101)
	}
```

------------
## 6: GN Connector 服务 样例及说明
#### 目前 Gn Connector 中仅仅支持 websocket 连接，并使用 github.com/gorilla/websocket 该包，connector 包中 主要是 管理前端websocket连接以及消息包的路由算法。和 连接验证等等一些方法 
```go 
// 样例
	config, err := config.NewConfig("../../config/development.json")
	if err != nil {
		logger.Infof("config  error ", err)
		return
	}
	// logger.Infof("config:      %v  \n  ", config)
	connector, err := connector.DefaultConnector(config)
	if err != nil {
		logger.Infof("new DefaultConnect  error ", err)
		return
	}
	// exception handler
	connector.AddExceptionHandler(func(exception *gnError.GnException) {
		// close handler push msg
		if exception.Exception == gnError.WS_CLOSED && len(exception.BindId) > 0 && len(exception.Id) > 0 {
			handlerName := "wsclose"
			serverAddress := connector.GetServerIdByRouter(handlerName, exception.BindId, exception.Id,
				config.GetServerByType("login"))
			connector.SendPack(serverAddress, handlerName, exception.BindId, exception.Id, nil)
		}
	})

	// set pack  route 路由消息 算法
	connector.AddRouterRearEndHandler("connector", connectorRoure)
	err = connector.Run()
	if err != nil {
		logger.Infof("connector run   error   %v \n", err)
		return
	}
	defer connector.Done()

	func connectorRoure(cid string, bindId string, serverList []*config.ServersConfig) string {
		if len(bindId) > 0 {
			index := int(crc32.ChecksumIEEE([]byte(bindId))) % len(serverList)
			if serverList[index] != nil {
				return serverList[index].ID
			}
		}
		return ""
	}
``` 


------------

## 7: Nats 消息中间件服务 说明
#### Nats  过多的 官话，我就不说了，请参考官网  https://nats.io/ ，GitHub:https://github.com/nats-io/go-nats

##### 1：下载对应系统的版本，
##### 2：因为是可执行文件，可以直接执行，./nats-server -DV 启动， 具体 其他配置以及配置文件  参考官网  
##### 3：GN框架主要用了，nats 的go 的客户端包  github.com/nats-io/nats.go
##### 4: 启动完毕 nats ，请把 nats的  URL 配置到  config 配置

------------
## 8: Config  说明 以及样例
 #### config 主要是 多个游戏服务的 服务配置， 不涉及到业务逻辑 ，业务逻辑 请参考 App.setConfig  的viper模块
 ```javascript 
	// 配置文件JSON说明 后续会增加 服务 所有ID 均需唯一，来区分唯一性
	{
		// nats  中间件 配置 ，所有消息包的中转，等等都需要用到 nats 
		// 暂时是必配选项
		"natsconfig":{
			"host":"nats://fpp:bar@localhost",
			"port":4222
		},
		// connector 服务器 配置，可以是多个，connector 
		// 根据NGINX或者其他 负载均衡到 对应 connector
		// 均为必配选项
		"connector":[
			{
			"id":"connector001",// ID必须 唯一
			"host": "0.0.0.0",
			"clientPort": 12007,
			"frontend": true, // 是否为前端 服务器
			"heartTime":5, // 心跳时间，和客户端 单位为秒
			"serverType":"connector"  // 服务类型 
			}
		],
		// 业务逻辑服务器 
		// Id 均需 唯一
		// serverType 相同 会 认为是一组相同类型的业务服务器  
		// 时间单位 均为秒
		"servers":[
			{
				"id":"login-001",
				"serverType":"login",
				"handleTimeOut":10, // handler 执行逻辑 超时时长
				"rpcTimeOut":5, 	// RPC  远程调用 超时 时间 
				"maxRunRoutineNum":10240 // 服务最大 同时执行业务 ，所开启的协程最大数量，超过最大数量会卡住 channel
			},
			{
				"id":"chat-001",
				"serverType":"chat",
				"handleTimeOut":10,
				"rpcTimeOut":5,
				"maxRunRoutineNum":10240
			}
		],
		// master 服务 非必配选择，
		// 如果不启动master 服务，可以不配置该 选项
		"master":{
			"id":"master",
			"nodeHeartBeart":10  // 服务子节点 失联 心跳  时间 10秒
		},
		//  log日志文件 配置 
		"log":{
			"encoding": "utf-8", // 编码
			"level":"All", 	// 日志级别 ，
			"maxLogSize": 10485760,// 日志文件 大小  超过该大小会 再次分割文件
			"numBackups":10 // 最大备份文件数量，超过 文件数，会重新回滚 覆盖
		}
	}
 ```

------------

# 快速上手及 demo

##### 获取 gnchatdemo  首先设置好 gopath   等相关 golang 环境，新手请自行百度
 


```
git clone https://github.com/wmyi/gnchatdemo
```

#### 1 docker-compose 单机部署
##### gnchatDemo 相应的docker文件已经写好， 需要 大家安装好 docker和docker-compose 插件 
##### 1):Mac 安装docker 会自动安装 docker-compose 插件
##### 2):win 需要大家自己安装 docker-compose 插件
##### 3):在gnchatDemo 目录 打开命令行 执行 docker-compose up  命令
##### 4):如果不报错，就OK 启动成功 （注意第一次可能会慢一些，因为 需要dockerhub ，拉去 相关镜像）
##### docker相关启动截图 <br>
![dockerps_start](./doc/img/dockerps_start.jpg "dockerps_start") <br>
##### 其中 NGINX服务 是 web静态网页 渲染用 <br>
##### 5):打开浏览器，输入  http://localhost:8080/#/ 如果弹出 让你输入昵称的 输入框，就证明启动OK <br>
##### 6):相关截图 <br>
![chat_1](./doc/img/chat_1.jpg "chat_1")<br>
![chat_2](./doc/img/chat_2.jpg "chat_2")<br>
![chat_3](./doc/img/chat_3.jpg "chat_3")<br>


#### 2 单个服务部署 需要 配置好golang 服务环境 和node 环境 因为 web前端界面渲染需要node环境
##### 1)：下载nats 对应系统版本 并启动  ./nats-server -DV  启动完成如下图 <br>
![nats_start](./doc/img/nats_start.jpg "nats_start") <br>
##### 2): 启动 connector ，login，chat，master等相关服务<br>
##### cd 进入 gnchatdemo/app/connector/ 目录下<br>
##### 执行 go run connector -serverId connector001  -mode debug  <br>
##### // serverId 是用来确认服务唯一性，必须唯一性，和配置文件里面的connector ID 必须一样 -mode debug 代表 log 输出到控住台，不输入该参数，默认写到配置文件所配置的 logs文件夹  启动成功如下 <br>
![connector_start](./doc/img/connector_start.png "connector_start")<br>
##### 其他服务 类似connector 逐个启动就OK，切记master  不需要 输入serverId  因为默认 集群就一个 master 所以省略了，直接 go run chatMaster.go  -mode debug 即可<br>
##### 也可以根据自己 需要选择是否 为debug 模式   -mode 为 可选项<br>
##### 3)：启动web服务 因为是node的服务，所以需要安装 npm 包
##### cd 进入 gnchatdemo/app/web/ 有package.json
##### 输入命令：  npm install   如果不报错，安装完所有依赖包
##### 输入 npm run dev  一切顺利  会提示 lisenter http://localhost:8080/
##### 在浏览器 输入 http://localhost:8080/ 弹出 输入昵称 输入框。<br>
##### 大功告成

 具体详情请参考 chatdemo 的readme.md #### [gnchatdemo](https://github.com/wmyi/gnchatdemo"gnchatdemo")
 #### 部署 GNchat demo  单机开发建议 用docker-compose  方便 多服务管理！
 #### 线上 可以根据 自己情况是否用docker，或者 直接启动二进制文件。都可以

 ------------
# API & interface 说明

