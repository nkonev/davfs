package main

import (
	"context"
	"errors"
	"flag"
	"github.com/nkonev/davfs"
	_ "github.com/nkonev/davfs/plugin/file"
	_ "github.com/nkonev/davfs/plugin/memory"
	_ "github.com/nkonev/davfs/plugin/mysql"
	_ "github.com/nkonev/davfs/plugin/postgres"
	_ "github.com/nkonev/davfs/plugin/sqlite3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/webdav"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	duration, e := time.ParseDuration("10s")
	if e != nil {
		panic("error during parsing duration")
	}
	var (
		addr   = flag.String("addr", ":9999", "server address")
		driver = flag.String("driver", "file", "database driver")
		source = flag.String("source", ".", "database connection string")
		cred   = flag.String("cred", "", "credential for basic auth")
		// todo deprecated
		create             = flag.Bool("create", false, "create filesystem")
		forceShutdownAfter = flag.Duration("force-shutdown-after", duration, "After interrupt signal handled wait this time before forcibly shut down https server")
		level              = flag.String("log-level", "INFO", "Might be TRACE, DEBUG, INFO, WARN, ERROR, FATAL")
		forceColors        = flag.Bool("force-colors", false, "Should force colors")
	)
	flag.Parse()

	log.SetReportCaller(true)
	formatter := log.TextFormatter{DisableLevelTruncation: true, ForceColors: *forceColors, FullTimestamp: true}
	log.SetFormatter(&formatter)
	log.SetOutput(os.Stdout)
	switch *level {
	case "TRACE":
		log.SetLevel(log.TraceLevel)
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "WARN":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "FATAL":
		log.SetLevel(log.FatalLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	var srv *http.Server
	if handler, e := createServer(driver, source, cred, create); e != nil {
		panic(e)
	} else {
		srv = runServer(addr, handler)
	}

	log.Println("Server started. Waiting for interrupt (2) (Ctrl+C)")
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 10 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Printf("Got signal %v - will forcibly close after %v\n", os.Interrupt, forceShutdownAfter)
	ctx, cancel := context.WithTimeout(context.Background(), *forceShutdownAfter)
	defer cancel() // releases resources if slowOperation completes before timeout elapses
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	} else {
		log.Println("Server successfully shut down")
	}

}

func createServer(driver, source, cred *string, create *bool) (http.Handler, error) {

	log.SetOutput(os.Stdout)

	if *create {
		err := davfs.CreateFS(*driver, *source)
		if err != nil {
			log.Fatal(err)
		}
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
				log.Printf("%s '%s' '%s'", r.Method, r.URL.Path, dst)
			default:
				log.Printf("%s '%s'", r.Method, r.URL.Path)
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
				log.Printf("authorization failed for user='%s'\n", username)
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

func runServer(addr *string, handler http.Handler) *http.Server {
	log.Printf("Server will started %v", *addr)

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	srv := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Println("Server will stopped due fatal error")
			log.Fatalf("ListenAndServe(): %s", err)
		}
		log.Println("Server stopped")
	}()

	return srv
}
