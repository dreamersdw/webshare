package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

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

	dir := strings.TrimPrefix(r.URL.Path, "/upload/")
	dst := path.Join(u.root, dir, path.Base(fileHeader.Filename))

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

	url := r.Header.Get("Referer")

	if url != "" {
		http.Redirect(w, r, url, http.StatusFound)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

type viewHandler struct {
	root string
	tmpl string
}

type navigation struct {
	Name   string
	Href   string
	IsLast bool
}

func buildNavigation(fullpath string, prefix string, rootName string) []navigation {
	if !strings.HasPrefix(fullpath, "/") {
		fullpath = "/" + fullpath
	}
	parts := strings.Split(fullpath, "/")

	nav := make([]navigation, len(parts))
	nav[0].Name = "Home"
	nav[0].Href = rootName + "/"
	nav[0].IsLast = false

	for i := 1; i < len(parts); i++ {
		nav[i].Name = parts[i]
		nav[i].Href = rootName + "/" + strings.Join(parts[0:i+1], "/")
		if i == len(parts)-1 {
			nav[i].IsLast = true
		} else {
			nav[i].IsLast = false
		}
	}
	return nav
}

type byName []os.FileInfo

func (f byName) Len() int {
	return len(f)
}

func (f byName) Swap(i int, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f byName) Less(i int, j int) bool {
	return f[i].Name() < f[j].Name()
}

func (v *viewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	html, _ := Asset(v.tmpl)
	t, err := template.New("").Parse(string(html))

	if err != nil {
		log.Warning("error %s", err)
	}

	files, err := ioutil.ReadDir(path.Join(v.root, r.URL.Path))

	sort.Sort(byName(files))

	t.Execute(w, struct {
		Title      string
		Path       string
		Navigation []navigation
		Files      []os.FileInfo
	}{
		"webshare",
		r.URL.Path,
		buildNavigation(r.URL.Path, "", "/ui"),
		files,
	})
}

func viewServer(root string, tmpl string) http.Handler {
	return &viewHandler{root, tmpl}
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
	http.Handle("/ui/", http.StripPrefix("/ui/", viewServer(dir, "static/template/view.html")))
	http.Handle("/upload/", uploadServer(dir))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(assetFS())))
	http.Handle("/", http.RedirectHandler("/ui/", http.StatusFound))
	http.ListenAndServe("0.0.0.0:8888", nil)
}
