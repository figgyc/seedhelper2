package main

import (
	"encoding/json"
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
var connections map[string]*websocket.Conn

// Device : struct for devices
type Device struct {
	FriendCode string
	ID0        string `bson:"_id"`
	HasMovable bool
	HasPart1   bool
	LFCS       []byte
}

// pool of devices for bot, and for bfers
var botFCs *mgo.Collection
var jobDevices *mgo.Collection
var workingDevices *mgo.Collection

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
	botFCs = mgoSession.DB("main").C("botFCs")
	jobDevices = mgoSession.DB("main").C("jobDevices")
	workingDevices = mgoSession.DB("main").C("workingDevices")

	// init templates
	view = jet.NewHTMLSet("./views")
	view.SetDevelopmentMode(true)

	// routing
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("home", make(jet.VarMap), r, w, nil)
	})

	// client:
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
			if messageType == websocket.TextMessage {
				var object map[string]interface{}
				err := json.Unmarshal(p, object)
				if err != nil {
					log.Println(err)
					return
				}
				if object["id0"] == nil {
					return
				}
				isRegistered := false
				for _, v := range connections {
					if v == conn {
						isRegistered = true
					}
				}
				if isRegistered == false {
					connections[object["id0"].(string)] = conn
				}

				if object["request"] == "bruteforce" {
					// add to BF pool
					query := jobDevices.Find(Device{ID0: object["id0"].(string)})
					count, err := query.Count()
					if err != nil {
						log.Println(err)
						return
					}
					if count > 1 {
						var device Device
						err = query.One(device)
						if err != nil {
							log.Println(err)
							return
						}
						if device.HasPart1 == true {
							err = jobDevices.Insert(device)
							if err != nil {
								log.Println(err)
								return
							}
						}
					} else {
						return
					}
				} else if object["friendCode"] != nil {
					// add to bot pool
					// TODO: verify fc
					device := Device{FriendCode: object["friendCode"].(string), ID0: object["id0"].(string)}
					err = devices.Insert(device)
					if err != nil {
						log.Println(err)
						return
					}
					err = botFCs.Insert(object["friendCode"].(string))
					if err != nil {
						log.Println(err)
						return
					}
					msg := "{\"status\": \"friendCodeAdded\"}"
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
						log.Println(err)
						return
					}
				} else {
					// checc
					query := jobDevices.Find(Device{ID0: object["id0"].(string)})
					count, err := query.Count()
					if err != nil {
						log.Println(err)
						return
					}
					if count > 1 {
						var device Device
						err = query.One(device)
						if err != nil {
							log.Println(err)
							return
						}
						if device.HasMovable == true {
							msg := "{\"status\": \"movablePart1\"}"
							if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
								log.Println(err)
								return
							}
						} else if device.HasPart1 == true {
							msg := "{\"status\": \"done\"}"
							if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
								log.Println(err)
								return
							}
						} else {
							if device.HasPart1 == true {
								msg := "{\"status\": \"friendCodeAdded\"}"
								if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
									log.Println(err)
									return
								}
							}
						}
					} else {
						return
					}
				}
			} else if messageType == websocket.CloseMessage {
				for k, v := range connections {
					if v == conn {
						delete(connections, k)
					}
				}
			}

		}
	})

	// part1 auto script:
	// /getfcs
	router.HandleFunc("/getfcs", func(w http.ResponseWriter, r *http.Request) {
		query := botFCs.Find(nil)
		count, err := query.Count()
		if err != nil || count < 1 {
			w.Write([]byte("nothing"))
			return
		}
		var fcs []string
		err = query.All(fcs)
		if err != nil || len(fcs) < 1 {
			w.Write([]byte("nothing"))
			return
		}

		for _, fc := range fcs {
			w.Write([]byte(fc))
			w.Write([]byte("\n"))
		}
		return
	})
	// /added/fc
	router.HandleFunc("/added/{fc}", func(w http.ResponseWriter, r *http.Request) {
		fc := mux.Vars(r)["fc"]

		query := jobDevices.Find(Device{FriendCode: fc})
		count, err := query.Count()
		if err != nil {
			log.Println(err)
			return
		}
		if count > 1 {
			var device Device
			err = query.One(device)
			if err != nil {
				log.Println(err)
				return
			}
			for id0, conn := range connections {
				if id0 == device.ID0 {
					msg := "{\"status\": \"friendCodeAdded\"}"
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
						log.Println(err)
						return
					}
				}
			}
		}
	})

	// /lfcs/fc
	// get param lfcs is lfcs as hex escaped eg %10%22%65%00 or whatevs
	router.HandleFunc("/lfcs/{fc}", func(w http.ResponseWriter, r *http.Request) {
		fc := mux.Vars(r)["fc"]
		lfcs, ok := r.URL.Query()["lfcs"]
		if ok == false {
			return
		}

		query := jobDevices.Find(Device{FriendCode: fc})
		count, err := query.Count()
		if err != nil {
			log.Println(err)
			return
		}
		if count > 1 {
			var device Device
			err = query.One(device)
			if err != nil {
				log.Println(err)
				return
			}
			device.LFCS = []byte(lfcs[0])
			err = devices.Insert(device)
			if err != nil {
				log.Println(err)
				return
			}
			err = jobDevices.Remove(Device{FriendCode: fc})
			if err != nil {
				log.Println(err)
				return
			}
			for id0, conn := range connections {
				if id0 == device.ID0 {
					msg := "{\"status\": \"movablePart1\"}"
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
						log.Println(err)
						return
					}
				}
			}
		}
	})

	// msed auto script:
	// /cancel/id0
	router.HandleFunc("/cancel/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		fmt.Println(id0)
	})
	// /getwork
	router.HandleFunc("/getwork", func(w http.ResponseWriter, r *http.Request) {

	})
	// /claim/id0
	router.HandleFunc("/claim/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		fmt.Println(id0)
	})
	// /part1/id0
	// this is also used by client if they want self BF
	router.HandleFunc("/part1/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		fmt.Println(id0)
	})
	// POST /upload/id0 w/ file movable
	router.HandleFunc("/upload/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		fmt.Println(id0)
	})

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("404error", make(jet.VarMap), r, w, nil)
	})

	fmt.Println("serving on :3000")
	http.ListenAndServe(":3000", router)
}
