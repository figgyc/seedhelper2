package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/CloudyKit/jet"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/kidstuff/mongostore"
	"gopkg.in/mgo.v2"
)

var view *jet.Set
var store *mongostore.MongoStore
var mgoSession mgo.Session
var devices *mgo.Collection

// Device : struct for devices
type Device struct {
	FriendCode string
	ID0        string `bson:"_id"`
	HasMovable bool
	HasPart1   bool
}

// pool of devices for bot, and for bfers
var botID0s *mgo.Collection
var jobID0s *mgo.Collection

func renderTemplate(template string, vars jet.VarMap, request *http.Request, writer http.ResponseWriter, context interface{}) {
	t, err := view.GetTemplate(template)
	if err != nil {
		panic(err)
	}
	if err = t.Execute(writer, vars, nil); err != nil {
		// error when executing template
		panic(err)
	}
	if err != nil {
		panic(err)
	}
}

func main() {
	// initialize mongo
	mgoSession, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer mgoSession.Close()
	store = mongostore.NewMongoStore(mgoSession.DB("sessions").C("sessions"), 86400, true, []byte(os.Getenv("SESSION_SECRET")))

	devices = mgoSession.DB("main").C("devices")
	botID0s = mgoSession.DB("main").C("botID0s")
	jobID0s = mgoSession.DB("main").C("jobID0s")

	// init templates
	view = jet.NewHTMLSet("./views")
	view.SetDevelopmentMode(true)

	// routing
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("home", make(jet.VarMap), r, w, nil)
	})

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	router.HandleFunc("/socket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		//... Use conn to send and receive messages.
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}
			if err := conn.WriteMessage(messageType, p); err != nil {
				log.Println(err)
				return
			}
		}
	})

	// /cancel/id0
	// /getwork
	// /part1/id0
	// /upload/id0

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("404error", make(jet.VarMap), r, w, nil)
	})

	fmt.Println("serving on :3000")
	http.ListenAndServe(":3000", router)
}
