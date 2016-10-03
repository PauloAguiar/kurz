package main

import (
	"net/http"
	"os"
	"path"

	"github.com/gorilla/mux"
	"github.com/pauloaguiar/kurz/lib"
	godis "github.com/simonz05/godis/redis"
)

func fileExists(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func static(w http.ResponseWriter, r *http.Request) {
	fname := mux.Vars(r)["fileName"]

	// empty means, we want to serve the index file. Due to a bug in http.serveFile
	// the file cannot be called index.html, anything else is fine.
	if fname == "" {
		fname = "index.htm"
	}

	staticDir := "static"
	staticFile := path.Join(staticDir, fname)

	if fileExists(staticFile) {
		http.ServeFile(w, r, staticFile)
	}
}

func main() {
	var (
		redis        *godis.Client
		server       *kurz.Kurz
		filenotfound string
		port         string
		host         string
		passwd       string
		listen       string
		hostname     string
	)

	port = "9999"
	listen = ""
	filenotfound = "www.google.com"

	if os.Getenv("HTTP_PLATFORM_PORT") != "" {
		port = os.Getenv("HTTP_PLATFORM_PORT")
	}

	hostname = "localhost:" + port

	if os.Getenv("DOMAIN_NAME") != "" {
		hostname = os.Getenv("DOMAIN_NAME")
	}

	if os.Getenv("LISTEN_ADDR") != "" {
		listen = os.Getenv("LISTEN_ADDR")
	}

	if os.Getenv("CUSTOMCONNSTR_REDIS.netaddress") != "" {
		host = os.Getenv("CUSTOMCONNSTR_REDIS.netaddress")
	}

	if os.Getenv("CUSTOMCONNSTR_REDIS.password") != "" {
		passwd = os.Getenv("CUSTOMCONNSTR_REDIS.password")
	}

	if os.Getenv("FILE_NOT_FOUND") != "" {
		filenotfound = os.Getenv("FILE_NOT_FOUND")
	}

	redis = godis.New(host, 0, passwd)

	server = kurz.NewKurz(redis, hostname, filenotfound)

	router := mux.NewRouter()

	router.HandleFunc("/shorten/{url:(.*$)}", server.Shorten)
	router.HandleFunc("/{short:([a-zA-Z0-9]+$)}", server.Resolve)
	router.HandleFunc("/{short:([a-zA-Z0-9]+)\\+$}", server.Info)
	router.HandleFunc("/info/{short:[a-zA-Z0-9]+}", server.Info)
	router.HandleFunc("/latest/{data:[0-9]+}", server.Latest)

	router.HandleFunc("/urllist/{fileName:(.*$)}", static)

	s := &http.Server{
		Addr:    listen + ":" + port,
		Handler: router,
	}

	s.ListenAndServe()
}
