package gemini  

import (
	"encoding/json"
	"errors"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/cvm/cvmagent/channel"
)

type libagent struct {
	protocol   string
	url        string
	conn       net.Conn
	ctlChannel channel.MessageChannel
}

func (l *libagent) Init() error {
	l.ctlChannel = channel.MessageChannel{}
	// open channel
	conn, err := net.Dial(l.protocol, l.url)
	if err != nil {
		log.Errorf("Open sock error: %s", err)
		return err
	}
	l.conn = conn

	if err := l.ctlChannel.Init(l.conn, l.conn); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (l *libagent) IsReady() {
	// receive ready message
	for {
		msg := <-l.ctlChannel.GetInputMessageChan()
		if msg.Type == channel.MSG_AGENT_READY {
			log.Info("Recv: MSG_AGENT_READY")
			break
		}
	}
}

func (l *libagent) SetIP(device, ip, mask string) error {
	msg := channel.Message{Type: channel.MSG_SET_IP,
		Content: channel.SetIPMessage{
			IfName:  device,
			IpAddr:  ip,
			NetMask: mask}}
	l.ctlChannel.SendMessage(msg)
	//receive ack
	for {
		msg := <-l.ctlChannel.GetInputMessageChan()
		if msg.Type == channel.MSG_ACK {
			log.Info("Recv: MSG_ACK")
			ackmsg := channel.AckMessage{}
			b, _ := json.Marshal(msg.Content)
			json.Unmarshal(b, &ackmsg)
			if ackmsg.AckType == channel.ACK_OK {
				log.Info("Set Ip success")
				return nil
			} else {
				log.Errorf("Set ip error: %s", ackmsg.AckMsg)
				return errors.New("Set ip error:" + ackmsg.AckMsg)
			}
		}
	}
}

func (l *libagent) AddContainer(rootfs string, cmdArgs []string, env []string) error {
	msg := channel.Message{Type: channel.MSG_ADD_CONTAINER,
		Content: channel.AddContainerMessage{
			Rootfs:  rootfs,
			CmdArgs: cmdArgs,
			Env:     env,
		},
	}
	l.ctlChannel.SendMessage(msg)

	        //receive ack
        for {
                msg := <-l.ctlChannel.GetInputMessageChan()
                if msg.Type == channel.MSG_ACK {
                        log.Info("Recv: MSG_ACK")
                        ackmsg := channel.AckMessage{}
                        b, _ := json.Marshal(msg.Content)
                        json.Unmarshal(b, &ackmsg)
                        if ackmsg.AckType == channel.ACK_OK {
                                log.Info("Add container success")
                                return nil
                        } else {
                                log.Errorf("Add container error: %s", ackmsg.AckMsg)
                                return errors.New("Add container error:" + ackmsg.AckMsg)
                        }       
                }       
        }
}

func (l *libagent) Destroy() error {
	return l.conn.Close()
}
