package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/op/go-logging"
)

var (
	log    = logging.MustGetLogger("main")
	format = logging.MustStringFormatter(
		"%{color}%{time:15:04:05.000} - %{level:.4s} %{color:reset} %{message}")
)

func setupLogging() {
	basicBackend := logging.NewLogBackend(os.Stdout, "", 1)
	formatedBackend := logging.NewBackendFormatter(basicBackend, format)
	leveledBackend := logging.SetBackend(formatedBackend)
	leveledBackend.SetLevel(logging.INFO, "")
	logging.SetBackend(leveledBackend)
}

type uploadHandler struct {
	root string
}

func uploadServer(root string) http.Handler {
	return &uploadHandler{root}
}

func (u *uploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	inFile, fileHeader, err := r.FormFile("file")
	if err != nil {
		msg := fmt.Sprintf("unable to parse http request, %s", err)
		log.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	dst := path.Join(u.root, path.Base(fileHeader.Filename))

	outFile, err := os.Create(dst)
	if err != nil {
		msg := fmt.Sprintf("error when create file %s", err)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	size, err := io.Copy(outFile, inFile)
	if err != nil {
		msg := fmt.Sprintf("unable to save file, %s", err)
		log.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
	}

	log.Info("upload file %s with size %d successfully\n", fileHeader.Filename, size)
	w.WriteHeader(http.StatusNoContent)
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
	http.Handle("/upload/", uploadServer(dir))
	http.ListenAndServe("0.0.0.0:8888", nil)
}
