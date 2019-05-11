package cmd

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	_ "github.com/google/uuid"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start bot services",
	Long:  "STARTING BOT SERVICES",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

var Listen string
var Cert string
var Key string
var Expire int
var Redis string

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVarP(&Listen, "listen", "l", "0.0.0.0:1337", "gobot server listen address and port")
	serverCmd.Flags().StringVarP(&Cert, "cert", "c", "cert.pem", "tls certificate")
	serverCmd.Flags().StringVarP(&Key, "key", "k", "key.pem", "tls key")
	serverCmd.Flags().StringVarP(&Redis, "redis", "r", "localhost:6379", "redis server")
	serverCmd.Flags().IntVarP(&Expire, "expire", "e", 30, "default redis expiration in seconds, this keeps client list fresh")
}

type Event struct {
	Id         int
	Timestamp  time.Time
	Action     string
	Parameters map[string]string
	Auth       string
}

func startServer() {
	cert, err := tls.LoadX509KeyPair(Cert, Key)
	checkError(err)
	config := tls.Config{Certificates: []tls.Certificate{cert}}

	pass, _ := password.Generate(64, 10, 10, false, false)
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	redisClient.Set("auth", pass, 0)
	log.Printf("password set in redis: %s", pass)

	now := time.Now()
	config.Time = func() time.Time { return now }
	config.Rand = rand.Reader

	service := Listen

	listener, err := tls.Listen("tcp", service, &config)
	checkError(err)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		go handleClient(conn)
	}
}

type Response struct {
	Id              int
	ResponseCode    int
	ResponseMessage string
	ResponseData    map[string]string
}

func processCheckin(conn net.Conn, event Event) {
	result := checkin(event)
	if result {
		response := Response{
			1,
			0,
			"checkin successful",
			map[string]string{},
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		log.Println(string(output))
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

func handleClient(conn net.Conn) {
	defer conn.Close()
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
	authed := checkAuth(event)
	if authed {
		log.Println(event.Action)
		switch event.Action {
		case "checkin":
			processCheckin(conn, event)
		case "register":
			processRegisterNode(conn, event)
		case "getjobs":
			processGetJobs(conn, event)
		case "deletejob":
			processDeleteJob(conn, event)
		}
	} else {
		conn.Close()
	}
}

func processDeleteJob(conn net.Conn, event Event) {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	if _, ok := event.Parameters["job"]; ok {
		result, err := redisClient.HDel("jobs:"+string(id), event.Parameters["job"]).Result()
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

func processGetJobs(conn net.Conn, event Event) {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.HGetAll("jobs:" + string(id)).Result()
	if err != nil {
		log.Println(err)
	}
	log.Printf("redis hgetall result: %s", result)
	log.Printf("length of result %d", len(result))
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

func checkAuth(event Event) bool {
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.Get("auth").Result()
	if err != nil {
		log.Println(err)
	}
	if result == "" {
		return false
	}
	if event.Auth == result {
		return true
	}
	return false
}

func checkin(event Event) bool {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	result, err := redisClient.Get("client:" + string(id)).Result()
	if err != nil {
		log.Println(err)
	}
	if len(result) == 0 {
		log.Printf("node not found: %d", event.Id)
		return false
	} else {
		return true
	}

}

func processRegisterNode(conn net.Conn, event Event) {
	id := strconv.Itoa(event.Id)
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	err := redisClient.HMSet("jobs:"+string(id), map[string]interface{}{
		"1234": "command",
		"2345": "command2",
		"3456": "command3",
	}).Err()
	if err != nil {
		log.Println(err)
	}
	expire := time.Duration(Expire) * time.Second
	err = redisClient.Set("client:"+string(id), 1, expire).Err()
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
	if err != nil {
		log.Println(err)
	}

}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}
