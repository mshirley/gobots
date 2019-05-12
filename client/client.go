package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/packr/v2"
	"log"
	"math/rand"
	"time"
)

type ClientConfig struct {
	Master   string
	Password string
	Random   bool
	ClientID int
	Name     string
	Wait     int
}

type Event struct {
	Id         int
	Timestamp  time.Time
	Action     string
	Parameters map[string]string
	Auth       string
}

type Response struct {
	Id              int
	ResponseCode    int
	ResponseMessage string
	ResponseData    map[string]string
}

func sendAndReceive(config *ClientConfig, event *Event) Response {
	tlsConfig := tls.Config{InsecureSkipVerify: true}
	client, err := tls.Dial("tcp", config.Master, &tlsConfig)
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

func processJobs(config *ClientConfig, jobResponse Response) {
	log.Printf("processing jobs: %s", jobResponse)
	if jobResponse.ResponseCode == 0 {
		deleteJob(config, jobResponse)
	}
}

func deleteJob(config *ClientConfig, jobResponse Response) {
	log.Printf("deleting job: %s", jobResponse)
	for i := range jobResponse.ResponseData {
		job := &Event{
			config.ClientID,
			time.Now(),
			"deletejob",
			map[string]string{
				"job": string(i),
			},
			config.Password,
		}
		result := sendAndReceive(config, job)
		log.Printf("delete job result: %s", result)

	}
}

func getJobs(config *ClientConfig, event *Event) Response {
	jobs := sendAndReceive(config, event)
	return jobs
}

func registerWithServer(config *ClientConfig, registration *Event) Response {
	response := sendAndReceive(config, registration)
	return response
}

func main() {
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
	if config.Random {
		rand.Seed(time.Now().UnixNano())
		clientID := rand.Intn(10000-1) + 1
		log.Printf("random id generated: %d", clientID)
	}

	log.Println(config.Master, config.Password)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered", r)
		}
	}()
	login := &Event{
		config.ClientID,
		time.Now(),
		"checkin",
		map[string]string{
			"param1": "none",
		},
		config.Password,
	}
	for {
		response := sendAndReceive(config, login)
		log.Println(response)
		if response.Id == 1 && response.ResponseCode == 1 {
			registration := &Event{
				config.ClientID,
				time.Now(),
				"register",
				map[string]string{
					"name":    config.Name,
					"details": "my details",
				},
				config.Password,
			}
			log.Println("registering with server")
			response = registerWithServer(config, registration)
			log.Println("registration complete")
		}
		log.Println(response.Id, response.ResponseCode)
		if response.Id == 1 && response.ResponseCode == 0 {
			log.Println("getting jobs")
			jobs := &Event{
				config.ClientID,
				time.Now(),
				"getjobs",
				map[string]string{
					"params": "all",
				},
				config.Password,
			}
			jobresult := getJobs(config, jobs)
			if len(jobresult.ResponseData) > 0 {
				processJobs(config, jobresult)
			}
		}
		wait := time.Duration(config.Wait) * time.Second
		time.Sleep(wait)
		log.Println("ended sleep")
	}
}
