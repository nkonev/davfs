package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/nkonev/gowebdav"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	test "net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	create = true
	body   = `
<?xml version="1.0" encoding="utf-8"?>
<propfind xmlns="DAV:"><prop>
<resourcetype xmlns="DAV:"/>
<getcontentlength xmlns="DAV:"/>
<getetag xmlns="DAV:"/>
<getlastmodified xmlns="DAV:"/>
<executable xmlns="http://apache.org/dav/props/"/>
</prop></propfind>
`
	authHeader = "Basic dXNlcjpwYXNzd29yZA=="

	postgresSource      = "host=localhost port=35432 user=webdav password=password dbname=webdav connect_timeout=2 statement_timeout=2000 sslmode=disable"
	postgresAdminSource = "host=localhost port=35432 user=postgres password=postgresqlPassword dbname=webdav connect_timeout=2 statement_timeout=2000 sslmode=disable"

	port        = "9998"
	davUser     = "user"
	davPassword = "password"
)

type testContext struct {
	driverName string
	source     string
}

var drivers = []testContext{
	{"memory", ""},
	{"postgres", postgresSource},
}

var counter int = 0

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	shutdown()
	os.Exit(retCode)
}

func setup() {
	const str = `
	DROP SCHEMA IF EXISTS public CASCADE;
    CREATE SCHEMA public;

    GRANT ALL ON SCHEMA public TO webdav;
    GRANT ALL ON SCHEMA public TO public;

    COMMENT ON SCHEMA public IS 'standard public schema';
`
	db, err := sql.Open("postgres", postgresAdminSource)
	if err != nil {
		log.Panicf("Got error in setup: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Panicf("Got error in setup: %v", err)
	}

	_, err2 := tx.Exec(str)
	if err2 != nil {
		log.Panicf("Got error in setup: %v", err2)
	}

	if err3 := tx.Commit(); err3 != nil {
		log.Panicf("Got error in setup: %v", err3)
	}
}

func shutdown() {

}

func authArgument() string {
	return davUser + ":" + davPassword
}

func runOnAllDrivers(t *testing.T, testCase func(tc testContext)) {
	for _, tt := range drivers {
		t.Run(tt.driverName, func(t *testing.T) {
			testCase(tt)
		})
	}
}

func TestGetCredFailedWithoutCredentials(t *testing.T) {

	runOnAllDrivers(t, func(tc testContext) {
		driver, source, cred, create := tc.driverName, tc.source, authArgument(), create

		handler, e := createServer(&driver, &source, &cred, &create)
		assert.Nil(t, e)

		req := test.NewRequest("PROPFIND", "/", strings.NewReader(body))
		w := test.NewRecorder()
		handler.ServeHTTP(w, req)

		resp := w.Result()

		body, _ := ioutil.ReadAll(resp.Body)

		fmt.Println(resp.StatusCode)
		fmt.Println(resp.Header.Get("Content-Type"))
		fmt.Println(string(body))
		assert.Equal(t, 401, resp.StatusCode)
	})

}

func TestGetCredFailedBadCredentials(t *testing.T) {
	runOnAllDrivers(t, func(tc testContext) {

		driver, source, cred, create := tc.driverName, tc.source, "user:password2", create

		handler, e := createServer(&driver, &source, &cred, &create)
		assert.Nil(t, e)

		req := test.NewRequest("PROPFIND", "/", strings.NewReader(body))
		header := map[string][]string{
			"Authorization": {authHeader},
		}
		req.Header = header
		w := test.NewRecorder()
		handler.ServeHTTP(w, req)

		resp := w.Result()

		body, _ := ioutil.ReadAll(resp.Body)

		fmt.Println(resp.StatusCode)
		fmt.Println(resp.Header.Get("Content-Type"))
		fmt.Println(string(body))
		assert.Equal(t, 401, resp.StatusCode)
	})
}

func TestGetCredSuccess(t *testing.T) {
	runOnAllDrivers(t, func(tc testContext) {

		driver, source, cred, create := tc.driverName, tc.source, authArgument(), create

		handler, e := createServer(&driver, &source, &cred, &create)
		assert.Nil(t, e)

		req := test.NewRequest("PROPFIND", "/", strings.NewReader(body))
		header := map[string][]string{
			"Authorization": {authHeader},
		}
		req.Header = header
		w := test.NewRecorder()
		handler.ServeHTTP(w, req)

		resp := w.Result()

		body, _ := ioutil.ReadAll(resp.Body)

		fmt.Println(resp.StatusCode)
		fmt.Println(resp.Header.Get("Content-Type"))
		fmt.Println(string(body))
		assert.Equal(t, 207, resp.StatusCode)
	})
}

func getUri() string {
	return "http://localhost:" + port
}

func generateRandomName(prefix string) string {
	counter++
	return prefix + strconv.Itoa(counter) + "_" + strconv.FormatInt(time.Now().Unix(), 10)
}

func getTempDirName() string {
	return generateRandomName("folder")
}

func getTempFileName() string {
	return generateRandomName("file")
}

func TestClientCreateDir(t *testing.T) {
	runOnAllDrivers(t, func(tc testContext) {
		driver, source, cred, create, addr := tc.driverName, tc.source, authArgument(), create, ":"+port

		handler, e := createServer(&driver, &source, &cred, &create)
		assert.Nil(t, e)

		ctx, cancel := context.WithCancel(context.Background())
		srv := runServer(&addr, handler)

		c := gowebdav.NewClient(getUri(), davUser, davPassword)
		_ = c.Connect() // performs authorization

		tempDir := getTempDirName()

		{
			err := c.Mkdir(tempDir, 0644)
			assert.Nil(t, err, "Got error %v", err)

			info, e2 := c.Stat(tempDir)
			assert.True(t, info.IsDir())
			assert.Nil(t, e2)
		}
		{
			// secondary call doesn' create directory and fails
			err := c.Mkdir(tempDir, 0644)
			assert.NotNil(t, err, "Got error %v", err)
		}

		assert.Nil(t, srv.Shutdown(ctx))
		cancel()
	})
}

func TestClientCreateFileInExistsDir(t *testing.T) {
	runOnAllDrivers(t, func(tc testContext) {
		driver, source, cred, create, addr := tc.driverName, tc.source, authArgument(), create, ":"+port

		handler, e := createServer(&driver, &source, &cred, &create)
		assert.Nil(t, e)

		ctx, cancel := context.WithCancel(context.Background())
		srv := runServer(&addr, handler)

		c := gowebdav.NewClient(getUri(), davUser, davPassword)
		_ = c.Connect() // performs authorization

		tempFile := getTempFileName()
		data := []byte("hello world file")

		var size1 int64
		{
			err := c.Write(tempFile, data, 0644)
			assert.Nil(t, err, "Got error %v", err)

			info, e2 := c.Stat(tempFile)
			assert.Nil(t, e2)

			assert.False(t, info.IsDir())
			size1 = info.Size()
		}
		{
			// secondary call doesn' create directory and fails
			err := c.Write(tempFile, data, 0644)
			assert.Nil(t, err, "Got error %v", err)

			info, e2 := c.Stat(tempFile)
			assert.Nil(t, e2)

			assert.Equal(t, size1, info.Size())
		}

		assert.Nil(t, srv.Shutdown(ctx))
		cancel()
	})
}

func TestClientCreateFileInNonExistsDir(t *testing.T) {
	runOnAllDrivers(t, func(tc testContext) {
		driver, source, cred, create, addr := tc.driverName, tc.source, authArgument(), create, ":"+port

		handler, e := createServer(&driver, &source, &cred, &create)
		assert.Nil(t, e)

		ctx, cancel := context.WithCancel(context.Background())
		srv := runServer(&addr, handler)

		c := gowebdav.NewClient(getUri(), davUser, davPassword)
		_ = c.Connect() // performs authorization

		tempFile := getTempDirName() + "/" + getTempFileName()
		data := []byte("hello world file")

		err := c.Write(tempFile, data, 0644)
		assert.NotNil(t, err, "Got error %v", err)

		info, e2 := c.Stat(tempFile)
		assert.NotNil(t, e2)
		assert.Nil(t, info)

		assert.Nil(t, srv.Shutdown(ctx))
		cancel()
	})
}
