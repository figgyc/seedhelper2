package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"github.com/CloudyKit/jet"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kidstuff/mongostore"
	"gopkg.in/mgo.v2"
	"net/http"
	"os"
	"time"
)

var view *jet.Set
var store *mongostore.MongoStore
var mgoSession mgo.Session
var users *mgo.Collection
var jobs *mgo.Collection

// User : Struct for users
type User struct {
	ID           uuid.UUID `bson:"_id"`
	Name         string
	Devices      []Device
	PasswordHash [72]byte
	FriendCode   string
	WorkPoints   int
}

// Device : struct for devices
type Device struct {
	ID          uuid.UUID `bson:"_id"`
	Name        string
	Owner       uuid.UUID
	FriendCode  string
	ID0         string
	HasMovable  bool
	HasPart1    bool
	Job         uuid.UUID `bson:",omitempty"`
	AutoMovable bool
}

// Job : Struct for work jobs
type Job struct {
	ID        uuid.UUID `bson:"_id"`
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	JobType   string
	WorkerID  uuid.UUID
	StartTime time.Time
	Active    bool
}

func renderTemplate(template string, vars jet.VarMap, request *http.Request, writer http.ResponseWriter, context interface{}) {
	session, err := store.Get(request, "session")
	if err != nil {
		// TODO: error handling
		panic(err)
	}
	t, err := view.GetTemplate(template)
	if err != nil {
		panic(err)
	}
	vars.Set("session", session.Values)
	vars.Set("flashes", session.Flashes())
	if session.Values["currentUser"] != nil {
		user := User{Name: session.Values["currentUser"].(string)}
		query := users.Find(user)
		query.One(user)
		vars.Set("user", user)
	} else {
		vars.Set("user", "no")
	}
	vars.Set("title", "Home")
	if err = t.Execute(writer, vars, nil); err != nil {
		// error when executing template
		panic(err)
	}
	err = session.Save(request, writer)
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

	users = mgoSession.DB("main").C("users")
	jobs = mgoSession.DB("main").C("jobs")

	// init templates
	view = jet.NewHTMLSet("./views")
	view.SetDevelopmentMode(true)

	// routing
	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session")
		if err != nil {
			panic(err)
		}
		if session.Values["currentUser"] != nil {
			renderTemplate("home", make(jet.VarMap), r, w, nil)
		} else {
			renderTemplate("index", make(jet.VarMap), r, w, nil)
		}
	})

	router.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("login", make(jet.VarMap), r, w, nil)
	}).Methods("GET")
	router.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session")
		if err != nil {
			panic(err)
		}
		err = r.ParseForm()
		if err != nil {
			panic(err)
		}
		user := User{Name: r.Form.Get("username")}
		if user.Name == "" {
			session.AddFlash("You must specify a username.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if r.Form.Get("password") == "" {
			session.AddFlash("You must specify a password.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		query := users.Find(user)
		count, err := query.Count()
		if err != nil {
			panic(err)
		}
		fmt.Println(query)
		if count != 1 {
			session.AddFlash("That user doesn't exist.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		query.One(user)
		passwordHash := sha512.Sum512([]byte(r.Form.Get("password")))
		if bytes.Equal(passwordHash[:], user.PasswordHash[8:]) == false {
			session.AddFlash("Incorrect password.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		session.AddFlash("Login successful.")
		session.Values["currentUser"] = user.Name
		err = session.Save(r, w)
		if err != nil {
			panic(err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)

	}).Methods("POST")

	router.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("register", make(jet.VarMap), r, w, nil)
	}).Methods("GET")
	router.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "session")
		if err != nil {
			panic(err)
		}
		err = r.ParseForm()
		if err != nil {
			panic(err)
		}
		id, err := uuid.NewRandom()
		if err != nil {
			panic(err)
		}
		user := User{Name: r.Form.Get("username"), ID: id}
		fmt.Println(r.Form, user.Name)
		if user.Name == "" {
			session.AddFlash("You must specify a username.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}
		if r.Form.Get("password") == "" {
			session.AddFlash("You must specify a password.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}
		if r.Form.Get("agree") != "on" {
			session.AddFlash("You must agree to the terms and privacy policy to make an account.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}
		query := users.Find(user)
		count, err := query.Count()
		if err != nil {
			panic(err)
		}
		if count != 0 {
			session.AddFlash("That user already exists.")
			err = session.Save(r, w)
			if err != nil {
				panic(err)
			}
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}
		randomBytes := make([]byte, 8)
		_, err = rand.Read(randomBytes)
		if err != nil {
			panic(err)
		}
		passwordHash := sha512.Sum512([]byte(r.Form.Get("password")))
		copy(user.PasswordHash[:], append(randomBytes, passwordHash[:]...))
		if err != nil {
			panic(err)
		}
		user.FriendCode = "0000-0000-0000"
		user.WorkPoints = 1
		err = users.Insert(user)
		if err != nil {
			panic(err)
		}
		session.AddFlash("Your account has been created.")
		session.Values["currentUser"] = user.Name
		err = session.Save(r, w)
		if err != nil {
			panic(err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}).Methods("POST")

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderTemplate("404error", make(jet.VarMap), r, w, nil)
	})

	fmt.Println("serving on :3000")
	http.ListenAndServe(":3000", router)
}
