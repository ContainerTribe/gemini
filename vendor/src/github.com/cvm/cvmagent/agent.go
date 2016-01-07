package main

import (
	"encoding/json"
	"os"
	"syscall"

	"github.com/cvm/cvmagent/channel"
	"github.com/cvm/cvmagent/runc"
	"github.com/opencontainers/runc/libcontainer"

	log "github.com/Sirupsen/logrus"
)

type CVMAgent struct {
	// message channel
	ctlChannel channel.MessageChannel

	//libcontainer factory
	factory libcontainer.Factory
}

func (c *CVMAgent) Run() {
	// init
	if err := c.init(); err != nil {
		log.Errorf("Init CVMAgent error: %s", err.Error())
		return
	}
	log.Info("Init CVMAgent success")

	// send ready message
	readyMessage := channel.Message{Type: channel.MSG_AGENT_READY, Content: channel.AgentReadyMessage{}}
	c.ctlChannel.SendMessage(readyMessage)

	// loop handle message
	go c.handleMessage()
}

func (c *CVMAgent) init() error {
	// mount basic filesystem
	if err := mountBasicFilesystem(); err != nil {
		return err
	}
	// FIXME: get serial port by name
	port, err := openSerialPort("/dev/vport2p1")
	if err != nil {
		return err
	}

	// init libcontainer
	c.factory, err = libcontainer.New("/cvmfs/", libcontainer.Cgroupfs)
	if err != nil {
		return err
	}

	// init control channel
	if err := c.ctlChannel.Init(port, port); err != nil {
		return err
	}

	return nil
}

func (c *CVMAgent) handleMessage() {
	for {
		msg := <-c.ctlChannel.GetInputMessageChan()
		switch msg.Type {
		case channel.MSG_ADD_CONTAINER:
			addcontainermsg := channel.AddContainerMessage{}
			b, _ := json.Marshal(msg.Content)
			json.Unmarshal(b, &addcontainermsg)
			log.Info("Recv: MSG_ADD_CONTAINER, Msg: %s", addcontainermsg)

			// mount
			os.Mkdir("/cvmfs", 0755)
			err := syscall.Mount("share_dir", "/cvmfs", "9p", 0, "trans=virtio")
			if err != nil {
				log.Errorf("Mount error: %s", err)
			}

			err = runc.CreateContainer(randomString(12),
				c.factory,
				addcontainermsg.Rootfs,
				addcontainermsg.CmdArgs,
				addcontainermsg.Env)
			if err != nil {
				c.sendAckMessage(channel.ACK_ERROR, err.Error())
				log.Errorf("Create container error: %s", err)
			} else {
				log.Info("Create container success!")
				c.sendAckMessage(channel.ACK_OK, "")
			}
		case channel.MSG_SET_IP:
			files, _ := listDir("/sys/class/net", "")
			log.Info(files)
			setipmsg := channel.SetIPMessage{}
			b, _ := json.Marshal(msg.Content)
			json.Unmarshal(b, &setipmsg)
			log.Infof("Recv:MSG_SET_IP, Msg: %s", setipmsg)

			// set ip
			err := setIp(setipmsg.IfName, setipmsg.IpAddr, setipmsg.NetMask)
			if err != nil {
				log.Errorf("Set ip error: %s", err)
				c.sendAckMessage(channel.ACK_ERROR, err.Error())
			} else {
				log.Info("Set ip success")
				c.sendAckMessage(channel.ACK_OK, "")
			}
		}
	}
}

func (c *CVMAgent) sendAckMessage(acktype int, msg string) {
	ackMessage := channel.Message{Type: channel.MSG_ACK,
		Content: channel.AckMessage{
			AckType: acktype,
			AckMsg:  msg}}
	c.ctlChannel.SendMessage(ackMessage)
}
