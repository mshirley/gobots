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
		log.Printf("connection error: %s", err)
	}
	jsonLogin, err := json.Marshal(event)
	if err != nil {
		log.Printf("json marshal error: %s", err)
	}
	_, err = client.Write([]byte(string(jsonLogin) + "\n"))
	if err != nil {
		log.Printf("client write error: %s", err)
	}
	r := bufio.NewReader(client)
	msg, err := r.ReadString('\n')
	if err != nil {
		response := &Response{
			0,
			1,
			"bufio error",
			map[string]string{},
		}
		log.Printf("bufio reader error, is the auth password set in redis?: %s", err)
		return *response
	}
	var response Response
	err = json.Unmarshal([]byte(msg), &response)
	if err != nil {
		log.Printf("response unmarshal error: %s", err)
	}
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
		if response.Id == 1 && response.ResponseCode == 1 {
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
			log.Println("registering with server")
			registerWithServer(registration)
		}
		if response.Id == 1 && response.ResponseCode == 0 {
			log.Println("getting jobs")
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
			if len(jobresult.ResponseData) > 0 {
				processJobs(jobresult)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func processJobs(jobResponse Response) {
	log.Printf("processing jobs: %s", jobResponse)
	if jobResponse.ResponseCode == 0 {
		deleteJob(jobResponse)
	}
}

func deleteJob(jobResponse Response) {
	log.Printf("deleting job: %s", jobResponse)
	for i := range jobResponse.ResponseData {
		job := &Event{
			2,
			time.Now(),
			"deletejob",
			map[string]string{
				"job": string(i),
			},
			"password",
		}
		result := sendAndReceive(job)
		log.Printf("delete job result: %s", result)

	}
}

func getJobs(event *Event) Response {
	jobs := sendAndReceive(event)
	return jobs
}

func registerWithServer(registration *Event) {
	_ = sendAndReceive(registration)
}
