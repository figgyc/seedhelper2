package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CloudyKit/jet"
	"github.com/Tomasen/realip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/acme/autocert"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var view *jet.Set
var mgoSession mgo.Session
var devices *mgo.Collection
var lastBotInteraction time.Time
var miners map[string]time.Time
var iminers map[string]time.Time
var ipPriority []string
var ipBlacklist []string
var botIP string
var connections map[string]*websocket.Conn

// Device : struct for devices
type Device struct {
	FriendCode uint64
	ID0        string `bson:"_id"`
	HasMovable bool
	HasPart1   bool
	HasAdded   bool
	WantsBF    bool
	LFCS       [8]byte
	MSed       [0x140]byte
	MSData     [12]byte
	ExpiryTime time.Time `bson:",omitempty"`
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func renderTemplate(template string, vars jet.VarMap, request *http.Request, writer http.ResponseWriter, context interface{}) {
	writer.Header().Add("Link", "</static/js/script.js>; rel=preload; as=script, <https://fonts.gstatic.com>; rel=preconnect, <https://fonts.googleapis.com>; rel=preconnect, <https://bootswatch.com>; rel=preconnect, <https://cdn.jsdelivr.net>; rel=preconnect,")
	t, err := view.GetTemplate(template)
	if err != nil {
		panic(err)
	}
	vars.Set("isUp", (lastBotInteraction.After(time.Now().Add(time.Minute * -5))))
	vars.Set("minerCount", len(miners))
	c, err := devices.Find(bson.M{"haspart1": true, "wantsbf": true, "expirytime": bson.M{"$ne": time.Now()}).Count()
	if err != nil {
		panic(err)
	}
	vars.Set("userCount", c)
	b, err := devices.Find(bson.M{"hasmovable": bson.M{"$ne": true}, "haspart1": true, "wantsbf": true, "expirytime": bson.M{"$ne": time.Time{}}}).Count()
	if err != nil {
		panic(err)
	}
	vars.Set("miningCount", b)
	a, err := devices.Find(bson.M{"haspart1": true}).Count()
	if err != nil {
		panic(err)
	}
	vars.Set("p1Count", a)
	z, err := devices.Find(bson.M{"hasmovable": true}).Count()
	if err != nil {
		panic(err)
	}
	vars.Set("msCount", z)
	n, err := devices.Count()
	if err != nil {
		panic(err)
	}
	vars.Set("totalCount", n)
	fmt.Println(miners, len(miners))
	if err = t.Execute(writer, vars, nil); err != nil {
		// error when executing template
		panic(err)
	}
	if err != nil {
		panic(err)
	}
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func filetypeFixer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		//log.Println(r)
		var tFile = regexp.MustCompile("\\.py$")
		if tFile.MatchString(r.RequestURI) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "inline")
		}
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}

func reverse(numbers []byte) {
	for i, j := 0, len(numbers)-1; i < j; i, j = i+1, j-1 {
		numbers[i], numbers[j] = numbers[j], numbers[i]
	}
}

func main() {
	lastBotInteraction = time.Now()
	miners = map[string]time.Time{}
	iminers = map[string]time.Time{}
	ipBlacklist = strings.Split(os.Getenv("SEEDHELPER_IP_BLACKLIST"), ",")
	ipPriority = strings.Split(os.Getenv("SEEDHELPER_IP_PRIORITY"), ",")
	botIP = os.Getenv("SEEDHELPER_BOT_IP")
	fmt.Println(ipBlacklist, ipPriority)
	// initialize mongo
	mgoSession, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer mgoSession.Close()

	devices = mgoSession.DB("main").C("devices")

	// init templates
	view = jet.NewHTMLSet("./views")
	view.SetDevelopmentMode(true)

	// routing
	router := mux.NewRouter()

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	router.Use(logger)
	router.Use(filetypeFixer)

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("home", make(jet.VarMap), r, w, nil)
	})

	router.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "logo.png")
	})

	// client:
	connections = make(map[string]*websocket.Conn)
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
				log.Println("disconnection?", err)
				return
			}
			if messageType == websocket.TextMessage {
				var object map[string]interface{}
				err := json.Unmarshal(p, &object)
				if err != nil {
					log.Println(err)
					//return
					for k, v := range connections {
						if v == conn {
							delete(connections, k)
						}
					}
					return
				}
				if object["id0"] == nil {
					//return
				}
				fmt.Println(object["part1"], "packet")
				/*isRegistered := false
				for _, v := range connections {
					if v == conn {
						isRegistered = true
					}
				}
				if isRegistered == false {*/
				connections[object["id0"].(string)] = conn
				//}

				if object["request"] == "bruteforce" {
					// add to BF pool
					err := devices.Update(bson.M{"_id": object["id0"].(string)}, bson.M{"$set": bson.M{"wantsbf": true, "expirytime": time.Time{}}})
					if err != nil {
						log.Println(err)
						//return
					}
				} else if object["request"] == "cancel" {
					// canseru jobbu
					err := devices.Remove(bson.M{"_id": object["id0"]})
					if err != nil {
						w.Write([]byte("error"))
						return
					}
				} else if object["part1"] != nil {
					// add to work pool
					valid := true
					if regexp.MustCompile("[0-9a-f]{32}").MatchString(object["id0"].(string)) == false {
						valid = false
					}

					if valid == false {
						msg := "{\"status\": \"friendCodeInvalid\"}"
						if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
							log.Println(err)
							return
						}
						continue
					}
					p1Slice, err := base64.StdEncoding.DecodeString(object["part1"].(string))
					if err != nil {
						log.Println(err)
						return
					}
					lfcsSlice := p1Slice[:8]
					reverse(lfcsSlice)
					var lfcsArray [8]byte
					copy(lfcsArray[:], lfcsSlice[:])
					device := bson.M{"lfcs": lfcsArray, "_id": object["id0"].(string), "haspart1": true, "hasadded": true}
					_, err = devices.Upsert(device, device)
					if err != nil {
						log.Println(err)
						//return
					}
					msg := "{\"status\": \"movablePart1\"}"
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
						log.Println(err)
						//return
					}
				} else if object["friendCode"] != nil {
					// add to bot pool
					/*
						based on https://github.com/ihaveamac/Kurisu/blob/master/addons/friendcode.py#L24
						    def verify_fc(self, fc):
								fc = int(fc.replace('-', ''))
								if fc > 0x7FFFFFFFFF:
									return None
								principal_id = fc & 0xFFFFFFFF
								checksum = (fc & 0xFF00000000) >> 32
								return (fc if hashlib.sha1(struct.pack('<L', principal_id)).digest()[0] >> 1 == checksum else None)
					*/
					valid := true
					fc, err := strconv.Atoi(object["friendCode"].(string))
					if err != nil {
						valid = false
					}
					if fc > 0x7FFFFFFFFF {
						valid = false
					}
					principalID := fc & 0xFFFFFFFF
					checksum := (fc & 0xFF00000000) >> 32

					pidb := make([]byte, 4)
					binary.LittleEndian.PutUint32(pidb, uint32(principalID))
					if int(sha1.Sum(pidb)[0])>>1 != checksum {
						valid = false
					}

					if regexp.MustCompile("[0-9a-f]{32}").MatchString(object["id0"].(string)) == false {
						valid = false
					}

					if valid == false {
						msg := "{\"status\": \"friendCodeInvalid\"}"
						if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
							log.Println(err)
							return
						}
						continue
					}
					fmt.Println(fc)
					device := bson.M{"friendcode": uint64(fc), "hasadded": false, "haspart1": false}
					_, err = devices.Upsert(bson.M{"_id": object["id0"].(string)}, device)
					if err != nil {
						log.Println(err)
						//return
					}
					msg := "{\"status\": \"friendCodeProcessing\"}"
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
						log.Println(err)
						//return
					}

				} else {
					// checc
					fmt.Println("check")
					query := devices.Find(bson.M{"_id": object["id0"].(string)})
					count, err := query.Count()
					if err != nil {
						log.Println(err)
						//return
					}
					if count > 0 {
						var device Device
						err = query.One(&device)
						if err != nil {
							log.Println(err)
							//return
						}
						if device.HasMovable == true {
							msg := "{\"status\": \"done\"}"
							if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
								log.Println(err)
								//return
							}
						} else if (device.ExpiryTime != time.Time{}) {
							msg := "{\"status\": \"bruteforcing\"}"
							if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
								log.Println(err)
								//return
							}
						} else if device.HasPart1 == true {
							msg := "{\"status\": \"movablePart1\"}"
							if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
								log.Println(err)
								//return
							}
						} else {
							msg := "{\"status\": \"friendCodeAdded\"}"
							if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
								log.Println(err)
								//return
							}
						}
					} else {
						fmt.Println("empty id0 to socket, dropped DB?")
						//return
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
		if realip.FromRequest(r) != botIP {
			w.Write([]byte("nothing"))
			return
		}
		lastBotInteraction = time.Now()

		query := devices.Find(bson.M{"hasadded": false})
		count, err := query.Count()
		if err != nil || count < 1 {
			w.Write([]byte("nothing"))
			return
		}
		var aDevices []Device
		err = query.All(&aDevices)
		if err != nil || len(aDevices) < 1 {
			w.Write([]byte("nothing"))
			return
		}
		for _, device := range aDevices {
			w.Write([]byte(strconv.FormatUint(device.FriendCode, 10)))
			w.Write([]byte("\n"))
		}
		return
	})
	// /added/fc
	router.HandleFunc("/added/{fc}", func(w http.ResponseWriter, r *http.Request) {
		if realip.FromRequest(r) != botIP {
			w.Write([]byte("fail"))
			return
		}
		b := mux.Vars(r)["fc"]
		a, err := strconv.Atoi(b)
		if err != nil {
			w.Write([]byte("fail"))
			log.Println(err)
			return
		}
		fc := uint64(a)

		fmt.Println(r, &r)

		err = devices.Update(bson.M{"friendcode": fc, "hasadded": false}, bson.M{"$set": bson.M{"hasadded": true}})
		if err != nil { // && err != mgo.ErrNotFound {
			w.Write([]byte("fail"))
			log.Println("a", err)
			return
		}

		query := devices.Find(bson.M{"friendcode": fc})
		var device Device
		err = query.One(&device)
		if err != nil {
			w.Write([]byte("fail"))
			log.Println("x", err)
			return
		}
		for id0, conn := range connections {
			//fmt.Println(id0, device.ID0, "hello!")
			if id0 == device.ID0 {
				msg := "{\"status\": \"friendCodeAdded\"}"
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					delete(connections, id0)
					//w.Write([]byte("fail"))
					log.Println(err)
					//return
				}
			}
		}
		w.Write([]byte("success"))

	})

	// /lfcs/fc
	// get param lfcs is lfcs as hex eg 34cd12ab or whatevs
	router.HandleFunc("/lfcs/{fc}", func(w http.ResponseWriter, r *http.Request) {
		if realip.FromRequest(r) != botIP {
			w.Write([]byte("fail"))
			return
		}
		b := mux.Vars(r)["fc"]
		a, err := strconv.Atoi(b)
		if err != nil {
			w.Write([]byte("fail"))
			log.Println(err)
			return
		}
		fc := uint64(a)

		lfcs, ok := r.URL.Query()["lfcs"]
		if ok == false {
			log.Println("wot")
			w.Write([]byte("fail"))
			return
		}

		sliceLFCS, err := hex.DecodeString(lfcs[0])
		if err != nil {
			w.Write([]byte("fail"))
			log.Println(err)
			return
		}
		var x [8]byte
		copy(x[:], sliceLFCS)
		x[0] = 0x00
		x[1] = 0x00
		x[2] = 0x00
		fmt.Println(fc, a, b, lfcs, x, sliceLFCS)
		err = devices.Update(bson.M{"friendcode": fc, "haspart1": false}, bson.M{"$set": bson.M{"haspart1": true, "lfcs": x}})
		if err != nil && err != mgo.ErrNotFound {
			w.Write([]byte("fail"))
			log.Println(err)
			return
		}

		query := devices.Find(bson.M{"friendcode": fc})
		var device Device
		err = query.One(&device)
		if err != nil {
			log.Println(err)
			w.Write([]byte("fail"))
			log.Println("las")
			return
		}
		for id0, conn := range connections {
			if id0 == device.ID0 {
				msg := "{\"status\": \"movablePart1\"}"
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					log.Println(err)
					delete(connections, id0)
					//w.Write([]byte("fail"))
					//return
				}
			}
		}

		w.Write([]byte("success"))
		log.Println("last")

	})

	// msed auto script:
	// /cancel/id0
	router.HandleFunc("/cancel/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		fmt.Println(id0)

		err := devices.Update(bson.M{"_id": id0}, bson.M{"$unset": bson.M{"expirytime": ""}})
		if err != nil {
			w.Write([]byte("error"))
			return
		}
		w.Write([]byte("success"))

	})
	// : 86.15.167.38
	// /getwork
	router.HandleFunc("/getwork", func(w http.ResponseWriter, r *http.Request) {
		miners[realip.FromRequest(r)] = time.Now()
		iminers[realip.FromRequest(r)] = time.Now()
		query := devices.Find(bson.M{"haspart1": true, "wantsbf": true, "expirytime": bson.M{"$eq": time.Time{}}})
		count, err := query.Count()
		if err != nil || count < 1 {
			w.Write([]byte("nothing"))
			return
		}
		var aDevice Device
		err = query.One(&aDevice)
		if err != nil {
			w.Write([]byte("nothing"))
			fmt.Println(err)
			return
		}
		w.Write([]byte(aDevice.ID0))
	})
	// /claim/id0
	router.HandleFunc("/claim/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		//fmt.Println(id0)
		err := devices.Update(bson.M{"_id": id0}, bson.M{"$set": bson.M{"expirytime": time.Now().Add(time.Hour)}})
		if err != nil {
			fmt.Println(err)
			return
		}
		w.Write([]byte("success"))
		miners[realip.FromRequest(r)] = time.Now()
		for id02, conn := range connections {
			//fmt.Println(id0, device.ID0, "hello!")
			if id02 == id0 {
				msg := "{\"status\": \"bruteforcing\"}"
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					delete(connections, id0)
					//w.Write([]byte("fail"))
					log.Println(err)
					//return
				}
			}
		}
	})
	// /part1/id0
	// this is also used by client if they want self BF so /claim is needed
	router.HandleFunc("/part1/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		query := devices.Find(bson.M{"_id": id0})
		count, err := query.Count()
		if err != nil || count < 1 {
			w.Write([]byte("error"))
			fmt.Println("z", err, count)
			return
		}
		var device Device
		err = query.One(&device)
		if err != nil || device.HasPart1 == false {
			w.Write([]byte("error"))
			fmt.Println("a", err)
			return
		}
		buf := bytes.NewBuffer(make([]byte, 0, 0x1000))
		leLFCS := make([]byte, 8)
		binary.BigEndian.PutUint64(leLFCS, binary.LittleEndian.Uint64(device.LFCS[:]))
		_, err = buf.Write(leLFCS)
		if err != nil {
			w.Write([]byte("error"))
			fmt.Println("b", err)
			return
		}
		_, err = buf.Write(make([]byte, 0x8))
		if err != nil {
			w.Write([]byte("error"))
			fmt.Println("c", err)
			return
		}
		_, err = buf.Write([]byte(device.ID0))
		if err != nil {
			w.Write([]byte("error"))
			fmt.Println("d", err)
			return
		}
		_, err = buf.Write(make([]byte, 0xFD0))
		if err != nil {
			w.Write([]byte("error"))
			fmt.Println("e", err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "inline; filename=\"movable_part1.sed\"")
		w.Write(buf.Bytes())
	})
	// /check/id0
	// allows user cancel and not overshooting the 1hr job max time
	router.HandleFunc("/check/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		query := devices.Find(bson.M{"_id": id0})
		count, err := query.Count()
		if err != nil || count < 1 {
			w.Write([]byte("error"))
			fmt.Println("z", err, count)
			return
		}
		var device Device
		err = query.One(&device)
		if err != nil || device.HasPart1 == false || device.HasMovable == true {
			w.Write([]byte("error"))
			fmt.Println("a", err)
			return
		}
		if device.WantsBF == false || device.ExpiryTime.Before(time.Now()) == true {
			w.Write([]byte("error"))
			return
		}
		miners[realip.FromRequest(r)] = time.Now()
		w.Write([]byte("ok"))
	})
	// /movable/id0
	router.HandleFunc("/movable/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		query := devices.Find(bson.M{"_id": id0})
		count, err := query.Count()
		if err != nil || count < 1 {
			fmt.Println(err)
			return
		}
		var device Device
		err = query.One(&device)
		if err != nil || device.HasMovable == false {
			w.Write([]byte("error"))
			return
		}
		buf := bytes.NewBuffer(make([]byte, 0, 0x140))
		_, err = buf.Write(device.MSed[:])
		if err != nil {
			fmt.Println(err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "inline; filename=\"movable.sed\"")
		w.Write(buf.Bytes())
	})
	// POST /upload/id0 w/ file movable and msed
	router.HandleFunc("/upload/{id0}", func(w http.ResponseWriter, r *http.Request) {
		id0 := mux.Vars(r)["id0"]
		file, header, err := r.FormFile("movable")
		if err != nil {
			fmt.Println(err)
			return
		}
		if header.Size != 0x120 && header.Size != 0x140 {
			w.WriteHeader(400)
			w.Write([]byte("error"))
			fmt.Println(header.Size)
			return
		}
		var movable [0x120]byte
		_, err = file.Read(movable[:])
		if err != nil {
			fmt.Println(err)
			return
		}
		err = devices.Update(bson.M{"_id": id0}, bson.M{"$set": bson.M{"msed": movable, "hasmovable": true, "expirytime": time.Time{}, "wantsbf": false}})
		if err != nil {
			fmt.Println(err)
			return
		}

		for key, conn := range connections {
			if key == id0 {
				msg := "{\"status\": \"done\"}"
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					log.Println(err)
					delete(connections, id0)
					//w.Write([]byte("fail"))
					//return
				}
			}
		}

		w.Write([]byte("success"))

		file2, header2, err := r.FormFile("msed")
		if header2.Size != 12 {
			fmt.Println(header.Size)
			return
		}
		var msed [12]byte
		_, err = file2.Read(msed[:])
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(header2.Filename)
		filename := "msed_data_" + id0 + ".bin"
		err = ioutil.WriteFile("static/mseds/"+filename, msed[:], 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
		f, err := os.OpenFile("static/mseds/list", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer f.Close()
		_, err = f.WriteString(filename + "\n")
		if err != nil {
			fmt.Println(err)
			return
		}

	}).Methods("POST")

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("404error", make(jet.VarMap), r, w, nil)
	})

	// anti abuse task
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("running task")
				query := devices.Find(bson.M{"expirytime": bson.M{"$ne": time.Time{}}})
				var theDevices []Device
				err := query.All(&theDevices)
				if err != nil {
					fmt.Println(err)
					return
				}
				for ip, miner := range miners {
					if miner.Before(time.Now().Add(time.Minute*-5)) == true {
						delete(miners, ip)
					}
				}
				for ip, miner := range iminers {
					if miner.Before(time.Now().Add(time.Second*-30)) == true {
						delete(iminers, ip)
					}
				}
				for _, device := range theDevices {
					if (device.ExpiryTime != time.Time{} || device.ExpiryTime.Before(time.Now()) == true) {
						err = devices.Update(device, bson.M{"$set": bson.M{"expirytime": time.Time{}, "wantsbf": false}})
						if err != nil {
							fmt.Println(err)
							return
						}

						for id0, conn := range connections {
							if id0 == device.ID0 {
								msg := "{\"status\": \"flag\"}"
								if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
									log.Println(err)
									delete(connections, id0)
									return
								}
							}
						}
						fmt.Println(device.ID0, "job has expired")
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	fmt.Println("serving on :80 and 443")
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("edge.figgyc.uk", "seedhelper.figgyc.uk"),
		Cache:      autocert.DirCache("."),
	}
	httpsSrv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      router,
	}
	httpsSrv.Addr = ":443"
	httpsSrv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
	go http.ListenAndServe(":80", m.HTTPHandler(router))
	httpsSrv.ListenAndServeTLS("", "")
}
