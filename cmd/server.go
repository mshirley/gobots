package cmd

import (
	"github.com/dgraph-io/badger"
	_ "github.com/google/uuid"
	"github.com/spf13/cobra"
	"log"
	"net"
	"net/http"
	"net/rpc"
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
	Message string
}

type RegEvent struct {
	Id   int
	Name string
}

type Task Event

func getDb() *badger.DB {
	opts := badger.DefaultOptions
	opts.Dir = "/tmp/badger"
	opts.ValueDir = "/tmp/badger"
	db, err := badger.Open(opts)
	if err != nil {
		log.Println(err)
	}
	return db
}

func (t *Task) GetNodes(args *Event, reply *Event) error {
	log.Println("get nodes event")
	db := getDb()
	defer db.Close()
	err := db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte("answer"), []byte("42"))
		return err
	})
	if err != nil {
		log.Println(err)
	}
	*reply = Event{
		"1337",
	}
	return nil
}

func (t *Task) Checkin(args *Event, reply *Event) error {
	log.Println("check in event")
	log.Println(args)
	db := getDb()
	defer db.Close()

	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("answer"))
		if err != nil {
			log.Println(err)
		}
		log.Println(item)
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	*reply = Event{
		"1337",
	}
	return nil
}

func (t *Task) RegisterNode(args *RegEvent, reply *Event) error {
	db := getDb()
	defer db.Close()
	log.Println("registering node")
	err := db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(string(args.Id)), []byte(args.Name))
		return err
	})
	if err != nil {
		log.Println()
	}
	*reply = Event{
		"1337",
	}
	return nil
}

func startServer() {
	task := new(Task)
	// Publish the receivers methods
	err := rpc.Register(task)
	if err != nil {
		log.Println("Format of service Task isn't correct. ", err)
	}
	// Register a HTTP handler
	rpc.HandleHTTP()
	// Listen to TPC connections on port 1234
	listener, e := net.Listen("tcp", ":1337")
	if e != nil {
		log.Println("Listen error: ", e)
	}
	log.Printf("Serving RPC server on port %d\n", 1337)
	// Start accept incoming HTTP connections
	err = http.Serve(listener, nil)
	if err != nil {
		log.Println("Error serving: ", err)
	}
}
