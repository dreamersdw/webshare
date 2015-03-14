package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/op/go-logging"
)

var (
	log    = logging.MustGetLogger("main")
	format = logging.MustStringFormatter(
		"%{color}%{time:15:04:05.000} %{shortfunc} - %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func setupLogging() {
	basicBackend := logging.NewLogBackend(os.Stdout, "", 1)
	formatedBackend := logging.NewBackendFormatter(basicBackend, format)
	leveledBackend := logging.SetBackend(formatedBackend)
	leveledBackend.SetLevel(logging.INFO, "")
	logging.SetBackend(leveledBackend)
}

func main() {
	setupLogging()
	log.Info("start webshare ...")
	dir, err := os.Getwd()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	http.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.Dir(dir))))
	http.ListenAndServe("0.0.0.0:8888", nil)
}
