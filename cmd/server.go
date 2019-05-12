package cmd

import (
	"bufio"
	randCrypto "crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gobuffalo/packr/v2"
	_ "github.com/google/uuid"
	"github.com/spf13/cobra"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	Master   string
	Redis    string
	Listen   string
	Password string
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start bot services",
	Long:  "STARTING BOT SERVICES",
	Run: func(cmd *cobra.Command, args []string) {
		box := packr.New("config", "./config")
		config := &ServerConfig{}

		s, _ := box.FindString("config.json")
		_ = json.Unmarshal([]byte(s), &config)
		fmt.Println(config)

		StartServer(*config)
		time.Sleep(1 * time.Second)
	},
}

var Listen string
var Cert string
var Key string
var Expire int
var Redis string
var Set bool

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVarP(&Listen, "listen", "l", "0.0.0.0:1337", "gobot server listen address and port")
	serverCmd.Flags().StringVarP(&Cert, "cert", "c", "cert.pem", "tls certificate")
	serverCmd.Flags().StringVarP(&Key, "key", "k", "key.pem", "tls key")
	serverCmd.Flags().StringVarP(&Redis, "redis", "r", "localhost:6379", "redis server")
	serverCmd.Flags().IntVarP(&Expire, "expire", "e", 30, "default redis expiration in seconds, this keeps client list fresh")
	serverCmd.Flags().BoolVarP(&Set, "setpass", "s", false, "set password and overwrite existing entry in redis")
}

type Event struct {
	Id         int
	Timestamp  time.Time
	Action     string
	Parameters map[string]string
	Auth       string
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
	str := b.String() // E.g. "ExcbsVQs"
	return str
}

func StartServer(config ServerConfig) {
	box := packr.New("pki", "./pki")
	boxCert, _ := box.FindString("cert.pem")
	boxKey, _ := box.FindString("key.pem")
	log.Println(boxCert, boxKey)
	cert, err := tls.X509KeyPair([]byte(boxCert), []byte(boxKey))
	checkError(err)
	tlsConfig := tls.Config{Certificates: []tls.Certificate{cert}}
	redisClient := redis.NewClient(&redis.Options{
		Addr: Redis,
	})
	defer redisClient.Close()
	result := redisClient.Get("auth")
	if result == nil || Set {
		pass := generatePassword()
		redisClient.Set("auth", pass, 0)
		log.Printf("password set in redis: %s", pass)
	} else {
		log.Printf("using existing password in redis: %s", result)
	}

	now := time.Now()
	tlsConfig.Time = func() time.Time { return now }
	tlsConfig.Rand = randCrypto.Reader

	service := Listen

	listener, err := tls.Listen("tcp", service, &tlsConfig)
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
	log.Println(event)
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
		log.Println("getting jobs")
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
	log.Println(event.Auth, result)
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
