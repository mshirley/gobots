package cmd

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/packr/v2"
	"github.com/spf13/cobra"
	"log"
	"math/rand"
	"time"
)

type ClientConfig struct {
	Master   string
	Password string
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "start bot client",
	Long:  "START BOT CLIENT",
	Run: func(cmd *cobra.Command, args []string) {
		for {
			box := packr.New("config", "./config")
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
			StartClient(*config)
		}
	},
}

var Master string
var ClientID int
var Random bool
var Name string
var Password string
var Wait int

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.Flags().StringVarP(&Master, "master", "m", "localhost:1337", "master gobot server")
	clientCmd.Flags().IntVarP(&ClientID, "id", "i", 2, "client id")
	clientCmd.Flags().IntVarP(&Wait, "wait", "w", 5, "check-in time in seconds")
	clientCmd.Flags().BoolVarP(&Random, "random", "r", true, "create random client id, overrides --id")
	clientCmd.Flags().StringVarP(&Name, "name", "n", "client", "client name")
	clientCmd.Flags().StringVarP(&Password, "password", "p", "password", "shared password to authenticate to master")
}

func sendAndReceive(event *Event) Response {
	config := tls.Config{InsecureSkipVerify: true}
	client, err := tls.Dial("tcp", Master, &config)
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
	log.Println(response)
	return response
}

func StartClient(config ClientConfig) {
	if config.Master != "" {
		Master = config.Master
	}
	if config.Password != "" {
		Password = config.Password
	}

	if Random {
		rand.Seed(time.Now().UnixNano())
		ClientID = rand.Intn(10000-1) + 1
		log.Printf("random id generated: %d", ClientID)
	}

	log.Println(Master, Password)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered", r)
		}
	}()
	login := &Event{
		ClientID,
		time.Now(),
		"checkin",
		map[string]string{
			"param1": "none",
		},
		Password,
	}
	for {
		response := sendAndReceive(login)
		log.Println(response)
		if response.Id == 1 && response.ResponseCode == 1 {
			registration := &Event{
				ClientID,
				time.Now(),
				"register",
				map[string]string{
					"name":    Name,
					"details": "my details",
				},
				Password,
			}
			log.Println("registering with server")
			response = registerWithServer(registration)
			log.Println("registration complete")
		}
		log.Println(response.Id, response.ResponseCode)
		if response.Id == 1 && response.ResponseCode == 0 {
			log.Println("getting jobs")
			jobs := &Event{
				ClientID,
				time.Now(),
				"getjobs",
				map[string]string{
					"params": "all",
				},
				Password,
			}
			jobresult := getJobs(jobs)
			if len(jobresult.ResponseData) > 0 {
				processJobs(jobresult)
			}
		}
		wait := time.Duration(Wait) * time.Second
		time.Sleep(wait)
		log.Println("ended sleep")
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
			ClientID,
			time.Now(),
			"deletejob",
			map[string]string{
				"job": string(i),
			},
			Password,
		}
		result := sendAndReceive(job)
		log.Printf("delete job result: %s", result)

	}
}

func getJobs(event *Event) Response {
	jobs := sendAndReceive(event)
	return jobs
}

func registerWithServer(registration *Event) Response {
	response := sendAndReceive(registration)
	return response
}
