package cherryConnector

import (
	"encoding/json"
	"github.com/phantacix/cherry/const"
	"github.com/phantacix/cherry/handler"
	"github.com/phantacix/cherry/interfaces"
	"github.com/phantacix/cherry/logger"
	"github.com/phantacix/cherry/net"
	"github.com/phantacix/cherry/net/message"
	"github.com/phantacix/cherry/net/pomelo_packet"
	"github.com/phantacix/cherry/profile"
	"github.com/phantacix/cherry/session"
	"github.com/phantacix/cherry/utils"
	"net"
)

type BlackListFunction func(list []string)

type CheckClientFunc func(typ string, version string) bool

type PomeloComponentOptions struct {
	Connector       cherryInterfaces.IConnector
	ConnectListener cherryInterfaces.IConnectListener
	PacketEncode    cherryInterfaces.PacketEncoder
	PacketDecode    cherryInterfaces.PacketDecoder
	Serializer      cherryInterfaces.ISerializer
	SessionOnClosed cherryInterfaces.SessionListener
	SessionOnError  cherryInterfaces.SessionListener

	BlackListFunc       BlackListFunction
	BlackList           []string
	ForwardMessage      bool
	Heartbeat           int
	DisconnectOnTimeout bool
	UseDict             bool
	UseProtobuf         bool
	UseCrypto           bool
	UseHostFilter       bool
	CheckClient         CheckClientFunc
	DataCompression     bool
}

type PomeloComponent struct {
	cherryInterfaces.BaseComponent
	PomeloComponentOptions
	connCount        *Connection
	sessionComponent *cherrySession.SessionComponent
	handlerComponent *cherryHandler.HandlerComponent
}

func NewPomelo() *PomeloComponent {
	s := &PomeloComponent{
		PomeloComponentOptions: PomeloComponentOptions{
			PacketEncode:        cherryPomeloPacket.NewEncoder(),
			PacketDecode:        cherryPomeloPacket.NewDecoder(),
			Serializer:          cherryNet.NewJSON(),
			BlackListFunc:       nil,
			BlackList:           nil,
			ForwardMessage:      false,
			Heartbeat:           30,
			DisconnectOnTimeout: false,
			UseDict:             false,
			UseProtobuf:         false,
			UseCrypto:           false,
			UseHostFilter:       false,
			CheckClient:         nil,
		},
		connCount: &Connection{},
	}
	return s
}

func NewPomeloWithOpts(opts PomeloComponentOptions) *PomeloComponent {
	return &PomeloComponent{
		PomeloComponentOptions: opts,
		connCount:              &Connection{},
	}
}

func (p *PomeloComponent) Name() string {
	return "connector.pomelo"
}

func (p *PomeloComponent) Init() {
}

func (p *PomeloComponent) AfterInit() {
	p.sessionComponent = p.App().Find(cherryConst.SessionComponent).(*cherrySession.SessionComponent)
	if p.sessionComponent == nil {
		panic("please preload session.handlerComponent.")
	}

	p.handlerComponent = p.App().Find(cherryConst.HandlerComponent).(*cherryHandler.HandlerComponent)
	if p.handlerComponent == nil {
		panic("please preload handler.handlerComponent.")
	}

	p.initHandshakeData()
	p.initHeartbeatData()

	// when new connect bind the session
	if p.ConnectListener != nil {
		p.Connector.OnConnect(p.ConnectListener)
	} else {
		p.Connector.OnConnect(p.initSession)
	}

	// new goroutine
	go p.Connector.Start()
}

func (p *PomeloComponent) initSession(conn net.Conn) {
	session := p.sessionComponent.Create(conn, nil) //TODO INetworkEntity
	p.connCount.IncreaseConn()

	//receive msg
	session.OnMessage(func(bytes []byte) error {
		packets, err := p.PacketDecode.Decode(bytes)
		if err != nil {
			cherryLogger.Warnf("bytes parse to packets error. session=[%s]", session)
			session.Closed()
			return nil
		}

		if len(packets) < 1 {
			cherryLogger.Warnf("bytes parse to Packets length < 1. session=[%s]", session)
			return nil
		}

		for _, pkg := range packets {
			if err := p.processPacket(session, pkg); err != nil {
				cherryLogger.Warn(err)
				return nil
			}
		}
		return nil
	})

	if p.SessionOnClosed == nil {
		session.OnClose(p.SessionOnClosed)
	}

	session.OnClose(func(_ cherryInterfaces.ISession) {
		p.connCount.DecreaseConn()
	})

	if p.SessionOnError == nil {
		session.OnError(p.SessionOnError)
	}

	//create a new goroutine to process read data for current socket
	session.Start()
}

func (p *PomeloComponent) processPacket(session cherryInterfaces.ISession, pkg *cherryInterfaces.Packet) error {
	switch pkg.Type {
	case cherryPomeloPacket.Handshake:
		if err := session.Send(hrd); err != nil {
			return err
		}
		session.SetStatus(cherryPomeloPacket.WAIT_ACK)

		cherryLogger.Debugf("[Handshake] session=[%session]", session)

	case cherryPomeloPacket.HandshakeAck:
		if session.Status() != cherryPomeloPacket.WAIT_ACK {
			cherryLogger.Warnf("[HandshakeAck] session=[%session]", session)
			session.Closed()
			return nil
		}

		session.SetStatus(cherryPomeloPacket.WORKING)
		if cherryProfile.Debug() {
			cherryLogger.Debugf("[HandshakeAck] session=[%session]", session)
		}

	case cherryPomeloPacket.Data:
		if session.Status() != cherryPomeloPacket.WORKING {
			return cherryUtils.ErrorFormat("[Msg] status error. session=[%session]", session)
		}

		msg, err := cherryNetMessage.Decode(pkg.Data)
		if err != nil {
			p.handleMessage(session, msg)
		}

	case cherryPomeloPacket.Heartbeat:
		d, err := p.PacketEncode.Encode(cherryPomeloPacket.Heartbeat, nil)
		if err != nil {
			return err
		}

		err = session.Send(d)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PomeloComponent) handleMessage(session cherryInterfaces.ISession, msg *cherryNetMessage.Message) {
	r, err := cherryNet.Decode(msg.Route)
	if err != nil {
		cherryLogger.Warnf("failed to decode route:%s", err.Error())
		return
	}

	if r.NodeType() == "" {
		//TODO ... remove this
		//r.NodeType = p.IAppContext().NodeType()
		return
	}

	if r.NodeType() == p.App().NodeType() {
		p.handlerComponent.InHandle(r, session, msg)
	} else {
		// forward to target node
	}
}

func (p *PomeloComponent) Stop() {
	if p.Connector != nil {
		p.Connector.Stop()
		p.Connector = nil
	}
}

var (
	// hbd contains the heartbeat packet data
	hbd []byte
	// hrd contains the handshake response data
	hrd []byte
)

func (p *PomeloComponent) initHandshakeData() {
	hData := map[string]interface{}{
		"code": 200,
		"sys": map[string]interface{}{
			"heartbeat":   p.Heartbeat,
			"dict":        cherryNetMessage.GetDictionary(),
			"ISerializer": "protobuf",
		},
	}
	data, err := json.Marshal(hData)
	if err != nil {
		panic(err)
	}

	if p.DataCompression {
		compressedData, err := cherryUtils.Compression.DeflateData(data)
		if err != nil {
			panic(err)
		}

		if len(compressedData) < len(data) {
			data = compressedData
		}
	}

	hrd, err = p.PacketEncode.Encode(cherryPomeloPacket.Handshake, data)
	if err != nil {
		panic(err)
	}
}

func (p *PomeloComponent) initHeartbeatData() {
	var err error
	hbd, err = p.PacketEncode.Encode(cherryPomeloPacket.Heartbeat, nil)
	if err != nil {
		panic(err)
	}
}
