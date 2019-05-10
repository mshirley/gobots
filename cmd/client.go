package cmd

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"time"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "start bot client",
	Long:  "START BOT CLIENT",
	Run: func(cmd *cobra.Command, args []string) {
		for {
			startClient()
			time.Sleep(1 * time.Second)
		}
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}

func sendAndReceive(event *Event) Response {
	config := tls.Config{InsecureSkipVerify: true}
	client, err := tls.Dial("tcp", "localhost:1337", &config)
	if err != nil {
		log.Println(err)
	}

	jsonLogin, err := json.Marshal(event)
	if err != nil {
		log.Println(err)
	}
	log.Println(string(jsonLogin))
	_, err = client.Write([]byte(string(jsonLogin) + "\n"))
	if err != nil {
		log.Println(err)
	}
	log.Println("reading from client")
	r := bufio.NewReader(client)
	msg, err := r.ReadString('\n')
	if err != nil {
		log.Println(err)
	}
	log.Println(msg)
	var response Response
	err = json.Unmarshal([]byte(msg), &response)
	if err != nil {
		log.Println(err)
	}
	log.Println(response)
	return response
}

func startClient() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered", r)
		}
	}()
	login := &Event{
		2,
		time.Now(),
		"checkin",
		map[string]string{
			"param1": "none",
		},
		"password",
	}
	for {
		response := sendAndReceive(login)
		log.Println(response)
		if response.ResponseCode == 1 {
			registration := &Event{
				2,
				time.Now(),
				"register",
				map[string]string{
					"name":    "bot01",
					"details": "my details",
				},
				"password",
			}
			registerWithServer(registration)
		}
		if response.ResponseCode == 0 {
			jobs := &Event{
				2,
				time.Now(),
				"getjobs",
				map[string]string{
					"params": "all",
				},
				"password",
			}
			jobresult := getJobs(jobs)
			log.Println(jobresult)
		}
		time.Sleep(5 * time.Second)
	}
}

func getJobs(event *Event) Response {
	jobs := sendAndReceive(event)
	return jobs
}

func registerWithServer(registration *Event) {
	response := sendAndReceive(registration)
	log.Println(response)

}
