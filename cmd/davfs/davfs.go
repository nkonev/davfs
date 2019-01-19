package main

import (
	"errors"
	"flag"
	"github.com/nkonev/davfs"
	_ "github.com/nkonev/davfs/plugin/file"
	_ "github.com/nkonev/davfs/plugin/memory"
	_ "github.com/nkonev/davfs/plugin/mysql"
	_ "github.com/nkonev/davfs/plugin/postgres"
	_ "github.com/nkonev/davfs/plugin/sqlite3"
	"golang.org/x/net/webdav"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func main() {
	var (
		addr   = flag.String("addr", ":9999", "server address")
		driver = flag.String("driver", "file", "database driver")
		source = flag.String("source", ".", "database connection string")
		cred   = flag.String("cred", "", "credential for basic auth")
		create = flag.Bool("create", false, "create filesystem")
	)
	flag.Parse()

	if handler, e := createServer(driver, source, cred, create); e != nil {
		panic(e)
	} else {
		runServer(addr, handler)
	}
}


func createServer(driver, source, cred *string, create *bool) (http.Handler, error) {

	log.SetOutput(os.Stdout)

	if *create {
		err := davfs.CreateFS(*driver, *source)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}
	fs, err := davfs.NewFS(*driver, *source)
	if err != nil {
		log.Fatal(err)
	}

	dav := &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			switch r.Method {
			case "COPY", "MOVE":
				dst := ""
				if u, err := url.Parse(r.Header.Get("Destination")); err == nil {
					dst = u.Path
				}
				log.Printf("%s %s", r.URL.Path, dst)
			default:
				log.Printf("%s", r.URL.Path)
			}
		},
	}

	var handler http.Handler
	if *cred != "" {
		token := strings.SplitN(*cred, ":", 2)
		if len(token) != 2 {
			flag.Usage()
			return nil, errors.New("Cannot parse credentials from commandline")
		}
		user, pass := token[0], token[1]
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok || username != user || password != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="davfs"`)
				http.Error(w, "authorization failed", http.StatusUnauthorized)
				return
			}
			dav.ServeHTTP(w, r)
		})
	} else {
		handler = dav
	}
	return handler, nil
}

func runServer(addr *string, handler http.Handler){
	http.Handle("/", handler)
	log.Printf("Server will started %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}