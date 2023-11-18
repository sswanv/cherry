package cherryActor

import (
	"reflect"

	"google.golang.org/protobuf/proto"

	ccode "github.com/cherry-game/cherry/code"
	cerror "github.com/cherry-game/cherry/error"
	creflect "github.com/cherry-game/cherry/extend/reflect"
	cutils "github.com/cherry-game/cherry/extend/utils"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cproto "github.com/cherry-game/cherry/net/proto"
)

func InvokeLocalFunc(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) {
	if app == nil {
		clog.Errorf("[InvokeLocalFunc] app is nil. [message = %+v]", m)
		return
	}

	EncodeLocalArgs(app, fi, m)

	values := make([]reflect.Value, 2)
	values[0] = reflect.ValueOf(m.Session) // session
	values[1] = reflect.ValueOf(m.Args)    // args
	fi.Value.Call(values)
}

func InvokeRemoteFunc(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) {
	if app == nil {
		clog.Errorf("[InvokeRemoteFunc] app is nil. [message = %+v]", m)
		return
	}

	EncodeRemoteArgs(app, fi, m)

	values := make([]reflect.Value, fi.InArgsLen)
	if fi.InArgsLen > 0 {
		values[0] = reflect.ValueOf(m.Args) // args
	}

	if m.IsCluster {
		cutils.Try(func() {
			rets := fi.Value.Call(values)
			rspCode, rspData := retValue(app.Serializer(), rets, app.ErrorHandler())

			retResponse(m.ClusterReply, &cproto.Response{
				Code: rspCode,
				Data: rspData,
			})

		}, func(errString string) {
			clog.Warnf("[InvokeRemoteFunc] invoke error. [message = %+v, err = %s]", m, errString)
			retResponse(m.ClusterReply, &cproto.Response{
				Code: ccode.RPCRemoteExecuteError,
			})
		})
	} else {
		cutils.Try(func() {
			if m.ChanResult == nil {
				fi.Value.Call(values)
			} else {
				rets := fi.Value.Call(values)
				rspCode, rspData := retValue(app.Serializer(), rets, app.ErrorHandler())
				m.ChanResult <- &cproto.Response{
					Code: rspCode,
					Data: rspData,
				}
			}
		}, func(errString string) {
			if m.ChanResult != nil {
				m.ChanResult <- nil
			}

			clog.Errorf("[remote] invoke error.[source = %s, target = %s -> %s, funcType = %v, err = %+v]",
				m.Source,
				m.Target,
				m.FuncName,
				fi.InArgs,
				errString,
			)
		})
	}
}

func EncodeRemoteArgs(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) error {
	if m.IsCluster {
		if fi.InArgsLen == 0 {
			return nil
		}

		return EncodeArgs(app, fi, 0, m)
	}

	return nil
}

func EncodeLocalArgs(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) error {
	return EncodeArgs(app, fi, 1, m)
}

func EncodeArgs(app cfacade.IApplication, fi *creflect.FuncInfo, index int, m *cfacade.Message) error {
	argBytes, ok := m.Args.([]byte)
	if !ok {
		return cerror.Errorf("Encode args error.[source = %s, target = %s -> %s, funcType = %v]",
			m.Source,
			m.Target,
			m.FuncName,
			fi.InArgs,
		)
	}

	argValue := reflect.New(fi.InArgs[index].Elem()).Interface()
	err := app.Serializer().Unmarshal(argBytes, argValue)
	if err != nil {
		return cerror.Errorf("Encode args unmarshal error.[source = %s, target = %s -> %s, funcType = %v]",
			m.Source,
			m.Target,
			m.FuncName,
			fi.InArgs,
		)
	}

	m.Args = argValue

	return nil
}

func retValue(serializer cfacade.ISerializer, rets []reflect.Value, handler func(error) int32) (int32, []byte) {
	var (
		retsLen = len(rets)
		rspCode = ccode.OK
		rspData []byte
	)

	if retsLen == 1 {
		if val := rets[0].Interface(); val != nil {
			switch v := val.(type) {
			case int32:
				rspCode = v
			case error:
				if val != nil {
					if handler != nil {
						rspCode = handler(v)
					} else {
						rspCode = ccode.UnknownError
					}
				}
			default:
				rspCode = ccode.UnknownError
			}
		}
	} else if retsLen == 2 {
		if !rets[0].IsNil() {
			data, err := serializer.Marshal(rets[0].Interface())
			if err != nil {
				rspCode = ccode.RPCRemoteExecuteError
				clog.Warn(err)
			} else {
				rspData = data
			}
		}

		if val := rets[1].Interface(); val != nil {
			switch v := val.(type) {
			case int32:
				rspCode = v
			case error:
				if handler != nil {
					rspCode = handler(v)
				}
			default:
				rspCode = ccode.UnknownError
			}
		}
	}

	return rspCode, rspData
}

func retResponse(reply cfacade.IRespond, rsp *cproto.Response) {
	if reply != nil {
		rspData, _ := proto.Marshal(rsp)
		err := reply.Respond(rspData)
		if err != nil {
			clog.Warn(err)
		}
	}
}
