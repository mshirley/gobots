package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"net/rpc"
	"time"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "start bot client",
	Long:  "START BOT CLIENT",
	Run: func(cmd *cobra.Command, args []string) {
		startClient()
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}

func getClient() *rpc.Client {
	log.Println("STARTING BOT CLIENT")
	serverAddress := "localhost"
	client, err := rpc.DialHTTP("tcp", serverAddress+":1337")
	if err != nil {
		log.Println("dialing:", err)
	}
	return client

}

func startClient() {
	client := getClient()
	args := &Event{
		"test",
	}
	go func() {
		for {
			var reply Event
			err := client.Call("Task.Checkin", args, &reply)
			if err != nil {
				log.Println(err)
			}
			log.Println(reply.Message)
			time.Sleep(1 * time.Second)
		}
	}()
	select {}
}
