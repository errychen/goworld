package main

import (
	"net"

	"fmt"

	"os"

	"time"

	"github.com/xiaonanln/goworld/common"
	"github.com/xiaonanln/goworld/components/dispatcher/dispatcher_client"
	"github.com/xiaonanln/goworld/config"
	"github.com/xiaonanln/goworld/consts"
	"github.com/xiaonanln/goworld/gwlog"
	"github.com/xiaonanln/goworld/netutil"
	"github.com/xiaonanln/goworld/proto"
)

type ClientProxy struct {
	*proto.GoWorldConnection
	clientid    common.ClientID
	filterProps map[string]string
}

func newClientProxy(netConn net.Conn, cfg *config.GateConfig) *ClientProxy {
	tcpConn := netConn.(*net.TCPConn)
	tcpConn.SetWriteBuffer(consts.CLIENT_PROXY_WRITE_BUFFER_SIZE)
	tcpConn.SetReadBuffer(consts.CLIENT_PROXY_READ_BUFFER_SIZE)

	var conn netutil.Connection = netutil.NetConnection{netConn}
	conn = netutil.NewBufferedReadConnection(conn)
	//if cfg.CompressConnection {
	// compressing connection, use CompressedConnection
	//conn = netutil.NewCompressedConnection(conn)
	//}

	gwc := proto.NewGoWorldConnection(conn, cfg.CompressConnection)
	return &ClientProxy{
		GoWorldConnection: gwc,
		clientid:          common.GenClientID(), // each client has its unique clientid
		filterProps:       map[string]string{},
	}
}

func (cp *ClientProxy) String() string {
	return fmt.Sprintf("ClientProxy<%s@%s>", cp.clientid, cp.RemoteAddr())
}

func (cp *ClientProxy) serve() {
	defer func() {
		cp.Close()
		// tell the gate service that this client is down
		gateService.onClientProxyClose(cp)
		if err := recover(); err != nil && !netutil.IsConnectionError(err.(error)) {
			gwlog.TraceError("%s error: %s", cp, err.(error))
			if consts.DEBUG_MODE {
				os.Exit(2)
			}
		} else {
			gwlog.Info("%s disconnected", cp)
		}
	}()

	for {
		var msgtype proto.MsgType_t
		cp.SetRecvDeadline(time.Now().Add(time.Millisecond * 50))
		pkt, err := cp.Recv(&msgtype)
		if pkt != nil {
			if msgtype == proto.MT_UPDATE_POSITION_YAW_FROM_CLIENT {
				cp.handleUpdatePositionYawFromClient(pkt)
			} else if msgtype == proto.MT_CALL_ENTITY_METHOD_FROM_CLIENT {
				cp.handleCallEntityMethodFromClient(pkt)
			} else {
				if consts.DEBUG_MODE {
					gwlog.TraceError("unknown message type from client: %d", msgtype)
					os.Exit(2)
				} else {
					gwlog.Panicf("unknown message type from client: %d", msgtype)
				}
			}

			pkt.Release()
		} else if err != nil && !netutil.IsTemporaryNetError(err) {
			panic(err)
		}

		cp.Flush()
	}
}

func (cp *ClientProxy) handleUpdatePositionYawFromClient(pkt *netutil.Packet) {
	pkt.AppendClientID(cp.clientid) // append clientid to the packet
	dispatcher_client.GetDispatcherClientForSend().SendPacket(pkt)
}

func (cp *ClientProxy) handleCallEntityMethodFromClient(pkt *netutil.Packet) {
	pkt.AppendClientID(cp.clientid) // append clientid to the packet
	dispatcher_client.GetDispatcherClientForSend().SendPacket(pkt)
}
