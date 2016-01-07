package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
)

func main(){
	log.Info("Begin to start CVM Agent")
	agent := CVMAgent{}

	agent.Run()

	for {
		time.Sleep(time.Second * 10)
		fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
	}
}
