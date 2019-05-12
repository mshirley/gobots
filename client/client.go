package main

import (
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/packr"
	"github.com/mshirley/gobots/cmd"
	"log"
)

type ClientConfig struct {
	Master   string
	Password string
}

func main() {
	box := packr.NewBox("./config")
	config := &ClientConfig{}

	s, err := box.FindString("config.json")
	if err != nil {
		log.Println(err)
	}
	err = json.Unmarshal([]byte(s), &config)
	if err != nil {
		log.Println(err)
	}
	fmt.Println(config)

	cmd.StartClient(cmd.ClientConfig(*config))
}
