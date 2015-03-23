package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/op/go-logging"
)

var (
	log    = logging.MustGetLogger("main")
	format = logging.MustStringFormatter(
		"%{color}%{time:15:04:05.000} - %{level:.4s} %{color:reset} %{message}")
	cfgPort = 8888
	cfgPath = "/"
)

const (
	version = "0.1"
	usage   = `Usage:
	webshare [--port=NUM] [PATH]
	webshare --version
	webshare --help

Example:
    webshare --port 8888 /var/log/`
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

func humanize(value int64, units []string) (float64, string) {
	index := int(math.Log(float64(value)) / math.Log(float64(1024)))
	if index >= len(units) {
		index = len(units) - 1
	} else if index < 0 {
		index = 0
	}
	return float64(value) / (math.Pow(float64(1024), float64(index))), units[index]
}

func humanizeBytes(value interface{}) string {
	n, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
	v, u := humanize(n, []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"})
	return fmt.Sprintf("%.1f %s", v, u)
}

func humanizeTime(value interface{}) string {
	t := value.(time.Time)
	return t.Format("2006-01-02 15:04:05")
}

func (v *viewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	html, _ := Asset(v.tmpl)
	funcMap := template.FuncMap{
		"humanizeBytes": humanizeBytes,
		"humanizeTime":  humanizeTime,
	}
	t, err := template.New("").Funcs(funcMap).Parse(string(html))

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

func promoteServerAddress(port int) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("====================================")
	for _, addr := range addrs {
		if strings.Contains(addr.String(), ":") {
			continue
		}

		parts := strings.Split(addr.String(), "/")

		ip := parts[0]

		if ip == "127.0.0.1" {
			continue
		}

		fmt.Printf("http://%s:%d\n", ip, port)
	}
	fmt.Println("====================================")
}

type AuditWriter struct {
	writer     http.ResponseWriter
	statusCode int
}

func (aw *AuditWriter) Header() http.Header {
	return aw.writer.Header()
}

func (aw *AuditWriter) Write(content []byte) (int, error) {
	return aw.writer.Write(content)
}

func (aw *AuditWriter) WriteHeader(code int) {
	aw.statusCode = code
	aw.writer.WriteHeader(code)
}

func (aw *AuditWriter) StatusCode() int {
	return aw.statusCode
}

func fromWriter(w http.ResponseWriter) http.ResponseWriter {
	return &AuditWriter{w, 200}

}

func Log(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aw := fromWriter(w)

		start := time.Now()
		handler.ServeHTTP(aw, r)
		end := time.Now()

		delta := float64(end.Sub(start).Nanoseconds()) / 1000000.0

		log.Info("%s %s %s %d in %.1f ms", r.RemoteAddr, r.Method, r.URL, aw.(*AuditWriter).StatusCode(), delta)
	})
}

func main() {
	opt, err := docopt.Parse(usage, nil, false, "", false, false)

	if err != nil {
		os.Exit(1)
	}

	if opt["--help"].(bool) {
		fmt.Println(usage)
		return
	}

	if opt["--version"].(bool) {
		fmt.Println(version)
		return
	}

	if opt["PATH"] != nil {
		cfgPath = opt["PATH"].(string)
	} else {
		dir, err := os.Getwd()

		if err != nil {
			fmt.Println("error when get working path", err)
			os.Exit(1)
		}
		cfgPath = dir
	}

	if opt["--port"] != nil {
		port, err := strconv.Atoi(opt["--port"].(string))

		if err != nil {
			fmt.Println("error when parse port")
			os.Exit(1)
		}

		cfgPort = port
	}

	setupLogging()

	address := fmt.Sprintf("0.0.0.0:%d", cfgPort)

	promoteServerAddress(cfgPort)

	log.Info("start webshare on %s ...", address)
	http.Handle("/fs/", http.StripPrefix("/fs/", http.FileServer(http.Dir(cfgPath))))
	http.Handle("/ui/", http.StripPrefix("/ui/", viewServer(cfgPath, "static/template/view.html")))
	http.Handle("/upload/", uploadServer(cfgPath))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(assetFS())))
	http.Handle("/", http.RedirectHandler("/ui/", http.StatusFound))
	e := http.ListenAndServe(address, Log(http.DefaultServeMux))
	if e != nil {
		log.Error("%s", e)
		os.Exit(1)
	}
}
