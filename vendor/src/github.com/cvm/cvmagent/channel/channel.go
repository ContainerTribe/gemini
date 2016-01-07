package channel

import (
	"encoding/json"
	"io"

	log "github.com/Sirupsen/logrus"
)

type MessageChannel struct {
	//
	reader io.Reader

	//
	writer io.Writer

	//
	inputMessageChan chan Message

	//
	outputMessageChan chan Message
}

type Ready struct {
	Name string
}

func (s *MessageChannel) Init(r io.Reader, w io.Writer) error {
	s.reader = r
	s.writer = w

	//
	s.inputMessageChan = make(chan Message, 128)
	s.outputMessageChan = make(chan Message, 128)

	// read message from reader
	go func() {
		dec := json.NewDecoder(s.reader)
		for {
			var msg Message
			if err := dec.Decode(&msg); err != nil {
				dec = json.NewDecoder(s.reader)
				continue
			}
			log.Infof("Recv msg: %s", msg)
			s.inputMessageChan <- msg
		}
	}()

	// send message to writer
	go func() {
		for {
			msg := <-s.outputMessageChan
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			log.Infof("Send msg: %s", msg)
			s.writer.Write(b)
		}
	}()
	return nil
}

func (s *MessageChannel) GetInputMessageChan() chan Message {
	return s.inputMessageChan
}

func (s *MessageChannel) GetOutputMessageChan() chan Message {
	return s.outputMessageChan
}

func (s *MessageChannel) SendMessage(msg Message) {
	s.outputMessageChan <- msg
}
