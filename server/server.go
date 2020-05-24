package main

import (
	"bufio"
	randCrypto "crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gobuffalo/packr/v2"
	_ "github.com/google/uuid"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	Redis  string `json:"redis"`
	Listen string `json:"listen"`
	Expire int    `json:"expire"`
}

type Event struct {
	Id         int
	Timestamp  time.Time
	Action     string
	Parameters map[string]string
	Auth       string
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

func generatePassword() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := 64
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	str := b.String()
	return str
}

func StartServer() {
	box := packr.New("config", "./config")
	config := &ServerConfig{}
	s, _ := box.FindString("config.json")
	_ = json.Unmarshal([]byte(s), &config)
	fmt.Println(config)
	box = packr.New("pki", "./pki")
	boxCert, _ := box.FindString("cert.pem")
	boxKey, _ := box.FindString("key.pem")
	log.Println(boxCert, boxKey)
	cert, err := tls.X509KeyPair([]byte(boxCert), []byte(boxKey))
	checkError(err)
	tlsConfig := tls.Config{Certificates: []tls.Certificate{cert}}
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.Get(redisClient.Context(), "auth").Result()
	if err == redis.Nil {
		pass := generatePassword()
		redisClient.Set(redisClient.Context(), "auth", pass, 0)
		log.Printf("password set in redis: %s", pass)
	} else {
		log.Printf("using existing password in redis: %s", result)
	}

	now := time.Now()
	tlsConfig.Time = func() time.Time { return now }
	tlsConfig.Rand = randCrypto.Reader

	service := config.Listen

	listener, err := tls.Listen("tcp", service, &tlsConfig)
	checkError(err)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		go handleClient(config, conn)
	}
}

type Response struct {
	Id              int
	ResponseCode    int
	ResponseMessage string
	ResponseData    map[string]string
}

func processCheckin(config *ServerConfig, conn net.Conn, event Event) {
	result := checkin(config, event)
	if result {
		response := Response{
			1,
			0,
			"checkin successful",
			map[string]string{},
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		_, err := conn.Write(output)
		if err != nil {
			log.Println(err)
		}

	} else {
		response := Response{
			1,
			1,
			"checkin failed",
			map[string]string{},
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		_, err := conn.Write(output)
		if err != nil {
			log.Println(err)
		}
	}
}

func handleClient(config *ServerConfig, conn net.Conn) {
	defer conn.Close()
	log.Println("connection info:", conn.LocalAddr(), conn.RemoteAddr())
	r := bufio.NewReader(conn)
	msg, err := r.ReadString('\n')
	if err != nil {
		log.Println(err)
		return
	}
	var event Event
	err = json.Unmarshal([]byte(msg), &event)
	if err != nil {
		log.Println(err)
	}
	log.Println(event)
	authed := checkAuth(config, event)
	if authed {
		log.Println(event.Action, event.Id, event.Parameters)
		switch event.Action {
		case "checkin":
			processCheckin(config, conn, event)
		case "register":
			processRegisterNode(config, conn, event)
		case "getjobs":
			processGetJobs(config, conn, event)
		case "deletejob":
			processDeleteJob(config, conn, event)
		case "jobresult":
			processJobResult(config, conn, event)
		}
	} else {
		conn.Close()
	}
}

func processJobResult(config *ServerConfig, conn net.Conn, event Event) {
	defer conn.Close()
	log.Println(event)
	response := Response{
		1,
		0,
		"job result received",
		map[string]string{
			"job": event.Parameters["job"],
		},
	}
	marshaled, _ := json.Marshal(response)
	output := []byte(string(marshaled) + "\n")
	_, _ = conn.Write(output)
}

func processDeleteJob(config *ServerConfig, conn net.Conn, event Event) {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis,
	})
	defer redisClient.Close()
	if _, ok := event.Parameters["job"]; ok {
		result, err := redisClient.HDel(redisClient.Context(), "jobs:"+id, event.Parameters["job"]).Result()
		if err != nil {
			log.Println(err)
		}
		if result == 1 {
			response := Response{
				1,
				0,
				"job deleted",
				map[string]string{
					"job": event.Parameters["job"],
				},
			}
			marshaled, _ := json.Marshal(response)
			output := []byte(string(marshaled) + "\n")
			_, _ = conn.Write(output)
		} else {
			response := Response{
				1,
				1,
				"job does not exist",
				map[string]string{
					"job": event.Parameters["job"],
				},
			}
			marshaled, _ := json.Marshal(response)
			output := []byte(string(marshaled) + "\n")
			_, _ = conn.Write(output)
		}

	}
}

func processGetJobs(config *ServerConfig, conn net.Conn, event Event) {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.HGetAll(redisClient.Context(), "jobs:"+id).Result()
	if err != nil {
		log.Println(err)
	}
	if len(result) == 0 {
		response := Response{
			1,
			1,
			"jobs",
			result,
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		_, _ = conn.Write(output)
	} else {
		response := Response{
			1,
			0,
			"jobs",
			result,
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		_, _ = conn.Write(output)
	}
}

func checkAuth(config *ServerConfig, event Event) bool {
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.Get(redisClient.Context(), "auth").Result()
	if err != nil {
		log.Println(err)
	}
	if result == "" {
		return false
	}
	log.Println(event.Auth, result)
	if event.Auth == result {
		return true
	}
	return false
}

func checkin(config *ServerConfig, event Event) bool {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.Get(redisClient.Context(), "client:"+id).Result()
	if err != nil {
		log.Println(err)
	}
	if result == "" {
		log.Printf("node not found: %d", event.Id)
		return false
	} else {
		return true
	}

}

func processRegisterNode(config *ServerConfig, conn net.Conn, event Event) {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.Redis,
	})
	defer redisClient.Close()
	expire := time.Duration(config.Expire) * time.Second
    info := &HostInfo{}
	_ = json.Unmarshal([]byte(event.Parameters["sysinfo"]), &info)
	log.Println("os is:", info.OS.Family)
	if info.OS.Family == "debian" {
		err := redisClient.HMSet(redisClient.Context(), "jobs:"+id, map[string]interface{}{
			"1234": "whoami",
			"2345": "ls /",
			"3456": "df",
			"4567": "cat /proc/cpuinfo",
		}).Err()
		if err != nil {
			log.Println(err)
		}
		redisClient.Expire(redisClient.Context(), "jobs:"+id, expire)
	}
	err := redisClient.Set(redisClient.Context(), "client:"+id, event.Parameters["sysinfo"], expire).Err()
	if err != nil {
		log.Println(err)
	}
	response := Response{
		1,
		0,
		"registration successful",
		map[string]string{},
	}
	marshaled, _ := json.Marshal(response)
	output := []byte(string(marshaled) + "\n")
	_, _ = conn.Write(output)
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}

func main() {
	StartServer()
}
