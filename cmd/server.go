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
		log.Println("client authenticated")
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
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if _, ok := event.Parameters["job"]; ok {
		result, err := redisClient.HDel(string(event.Id)+":jobs", event.Parameters["job"]).Result()
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
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	result, err := redisClient.HGetAll(string(event.Id) + ":jobs").Result()
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
	if len(result) == 0 {
		log.Printf("node not found: %d", event.Id)
		return false
	} else {
		return true
	}

}

func processRegisterNode(conn net.Conn, event Event) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	err := redisClient.HMSet(string(event.Id)+":jobs", map[string]interface{}{
		"1234": "command",
		"2345": "command2",
		"3456": "command3",
	}).Err()
	if err != nil {
		log.Println(err)
	}
	err = redisClient.Set(string(event.Id), 1, 0).Err()
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
