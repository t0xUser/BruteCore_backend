package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/browser"

	"api.brutecore/libs/lib_db"
	"api.brutecore/libs/lib_env"
	"api.brutecore/libs/lib_jwt"

	eg "api.brutecore/internal/engine"
	handler "api.brutecore/internal/handler"
	server "api.brutecore/internal/server"
)

const (
	banner = `
	.____                _           _____
	|  _ \              | |         / ____|
	| |_) | _ __  _   _ | |_   ___ | |       ___   _ __   ___
	|  _ < | '__|| | | || __| / _ \| |      / _ \ | '__| / _ \
	| |_) || |   | |_| || |_ |  __/| |____ | (_) || |   |  __/
	|____/ |_|    \__,_| \__| \___| \_____| \___/ |_|    \___| v1.0.0.1

	===================================================================
	`
)

func init() {
	fmt.Println(banner)
}

func openBrowser(port string) {
	if strings.ToUpper(runtime.GOOS) != "LINUX" {
		time.Sleep(time.Second * 2)
		_ = browser.OpenURL(fmt.Sprintf("http://127.0.0.1%s/", port))
	}
}

func main() {
	/*--- Инициализация конфгурации ---*/
	conf, err := lib_env.New()
	if err != nil {
		log.Fatalf("Error initializing configuration: %v", err)
	} else {
		log.Println("Configuration initalized")
	}
	defer conf.Close()

	/*--- Инициализация БД ---*/
	db, err := lib_db.New(lib_db.SQLite, conf.Database.Path, conf.Logs.DB)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	} else {
		log.Printf(`Database "%s" initalized`, conf.Database.Path)
	}
	defer db.Close()

	/*--- Инициализация JWT класса ---*/
	jwt, err := lib_jwt.New(lib_jwt.MapStrToMethod(conf.Jwt.Method), []byte(conf.Jwt.Key), conf.Jwt.AccessTime, conf.Jwt.RefreshTime)
	if err != nil {
		log.Fatalf("Error initializing JWT: %v", err)
	} else {
		log.Printf(`JWT with key "%s" initalized`, conf.Jwt.Key)
	}

	/*--- Инициализация движка ---*/
	pull := eg.NewPulling(db, time.Second*5)
	go pull.StartListen()

	app := server.New(conf)
	app.SetMiddleware()
	app.SetHandlers(handler.New(conf, db, jwt, pull))

	go openBrowser(conf.Http.Port)
	log.Printf(`Server initalized on port "%s" with group "%s"`, conf.Http.Port, conf.Http.Group)
	if err = app.Listen(); err != nil {
		log.Fatalf("Error initalizing the server: %v", err)
	}
}
