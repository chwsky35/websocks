package core

import (
	"io"
	"net"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("core")

type Client struct {
	LogLevel   loggo.Level
	ListenAddr *net.TCPAddr
	URL        *url.URL

	Mux   bool
	MuxWS *MuxWebSocket

	Dialer *websocket.Dialer

	CreatedAt time.Time
}

func (client *Client) Listen() (err error) {
	logger.SetLogLevel(client.LogLevel)

	listener, err := net.ListenTCP("tcp", client.ListenAddr)
	if err != nil {
		return err
	}

	logger.Infof("Start to listen at %s", client.ListenAddr.String())

	defer listener.Close()

	if client.Mux {
		err := client.OpenMux()
		if err != nil {
			logger.Debugf(err.Error())
			return err
		}

		go client.MuxWS.ClientListen()
	}

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			logger.Debugf(err.Error())
			continue
		}

		go client.handleConn(conn)
	}

	return nil
}

func (client *Client) handleConn(conn *net.TCPConn) {
	defer conn.Close()

	conn.SetLinger(0)

	err := handShake(conn)
	if err != nil {
		logger.Errorf(err.Error())
		return
	}

	_, host, err := getRequest(conn)
	if err != nil {
		logger.Errorf(err.Error())
		return
	}

	_, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43})
	if err != nil {
		logger.Errorf(err.Error())
		return
	}

	if client.Mux {
		client.DialMuxConn(host, conn)
	} else {
		client.DialWSConn(host, conn)
	}

	return
}

func (client *Client) DialWSConn(host string, conn *net.TCPConn) {
	wsConn, _, err := client.Dialer.Dial(client.URL.String(), map[string][]string{
		"WebSocks-Host": {host},
	})

	if err != nil {
		return
	}

	logger.Debugf("dialed ws for %s", host)

	ws := &WebSocket{
		conn: wsConn,
	}

	go func() {
		_, err = io.Copy(ws, conn)
		if err != nil {
			logger.Debugf(err.Error())
			return
		}
		return
	}()

	_, err = io.Copy(conn, ws)
	if err != nil {
		logger.Debugf(err.Error())
		return
	}
	return
}
