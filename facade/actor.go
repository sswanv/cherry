package cherryFacade

import (
	creflect "github.com/cherry-game/cherry/extend/reflect"
)

type (
	IActorSystem interface {
		GetIActor(id string) (IActor, bool)
		CreateActor(id string, handler IActorHandler) (IActor, error)
		PostRemote(m *Message) bool
		PostLocal(m *Message) bool
		PostEvent(data IEventData)
		SetLocalInvoke(invoke InvokeFunc)
		SetRemoteInvoke(invoke InvokeFunc)
		Call(source, target, funcName string, arg interface{}) error
		CallWait(source, target, funcName string, arg interface{}, reply interface{}) error
	}

	InvokeFunc func(app IApplication, fi *creflect.FuncInfo, m *Message)

	IActor interface {
		App() IApplication
		ActorID() string
		Path() *ActorPath
		Call(targetPath, funcName string, arg interface{}) error
		CallWait(targetPath, funcName string, arg interface{}, reply interface{}) error
	}

	IActorHandler interface {
		AliasID() string                          // actorId
		OnInit()                                  // 当Actor启动前触发该函数
		OnStop()                                  // 当Actor停止前触发该函数
		OnLocalReceived(m *Message) (bool, bool)  // 当Actor接收local消息时触发该函数
		OnRemoteReceived(m *Message) (bool, bool) // 当Actor接收remote消息时执行的函数
		OnFindChild(m *Message) (IActor, bool)    // 当actor查找子Actor时触发该函数
		Exit()                                    // 执行Actor退出
	}

	IActorChild interface {
		Create(id string, handler IActorHandler) (IActor, error) // 创建子Actor
		Get(id string) (IActor, bool)                            // 获取子Actor
		Remove(id string)                                        // 称除子Actor
		Each(fn func(i IActor))                                  // 遍历所有子Actor
	}
)

type (
	IEventData interface {
		Name() string     // 事件名
		SenderID() string // 发送者ActorID
	}
)