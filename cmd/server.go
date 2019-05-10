package cmd

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	_ "github.com/google/uuid"
	"github.com/spf13/cobra"
	"log"
	"net"
	"os"
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

func init() {
	rootCmd.AddCommand(serverCmd)
}

type Event struct {
	Id         int
	Timestamp  time.Time
	Action     string
	Parameters map[string]string
	Auth       string
}

func startServer() {
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	checkError(err)
	config := tls.Config{Certificates: []tls.Certificate{cert}}

	now := time.Now()
	config.Time = func() time.Time { return now }
	config.Rand = rand.Reader

	service := "0.0.0.0:1337"

	listener, err := tls.Listen("tcp", service, &config)
	checkError(err)
	log.Println("Listening")
	for {
		log.Println("accepting connection")
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
}

type Job struct {
	Id      int
	Message string
}

func processCheckin(conn net.Conn, event Event) {
	result := checkin(event)
	log.Println(result)
	if result {
		response := Response{
			1,
			0,
			"checkin successful",
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		log.Println(output)
		_, err := conn.Write(output)
		if err != nil {
			log.Println(err)
		}

	} else {
		response := Response{
			1,
			1,
			"checkin failed",
		}
		marshaled, _ := json.Marshal(response)
		output := []byte(string(marshaled) + "\n")
		log.Println(output)
		_, err := conn.Write(output)
		if err != nil {
			log.Println(err)
		}
	}
}

func handleClient(conn net.Conn) {
	r := bufio.NewReader(conn)
	msg, err := r.ReadString('\n')
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(msg)
	var event Event
	err = json.Unmarshal([]byte(msg), &event)
	if err != nil {
		log.Println(err)
	}
	log.Println(event)
	log.Println(event.Action)
	authed := checkAuth(event)
	if authed {
		log.Println("client authenticated")
		switch event.Action {
		case "checkin":
			processCheckin(conn, event)
		case "register":
			processRegisterNode(conn, event)
		case "getjobs":
			processGetJobs(conn, event)
		}
	} else {
		log.Println("client not authed")
		conn.Close()
	}
}

func processGetJobs(conn net.Conn, event Event) {
	log.Println(event)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	result, err := redisClient.Get(string(event.Id)).Result()
	if err != nil {
		log.Println(err)
	}
	log.Println(result)
	response := Response{
		1,
		0,
		"",
	}
	marshaled, _ := json.Marshal(response)
	output := []byte(string(marshaled) + "\n")
	log.Println(output)
	conn.Write(output)
	if err != nil {
		log.Println(err)
	}
}

func checkAuth(event Event) bool {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
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
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	result, err := redisClient.Get(string(event.Id)).Result()
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

func processRegisterNode(conn net.Conn, event Event) {
	log.Println(event)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	err := redisClient.Set(string(event.Id), 1, 0).Err()
	if err != nil {
		log.Println(err)
	}
	response := Response{
		1,
		0,
		"registration successful",
	}
	marshaled, _ := json.Marshal(response)
	output := []byte(string(marshaled) + "\n")
	log.Println(output)
	conn.Write(output)
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
