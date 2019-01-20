package main

import (
	"context"
	"fmt"
	"github.com/nkonev/gowebdav"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	test "net/http/httptest"
	"strings"
	"testing"
)

const (
	create = false
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
)

type testContext struct {
	driverName string
	source     string
}

var drivers = []testContext{
	{"memory", ""},
	{"postgres", "host=localhost port=35432 user=webdav password=password dbname=webdav connect_timeout=2 statement_timeout=2000 sslmode=disable"},
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
		driver, source, cred, create := tc.driverName, tc.source, "user:password", create

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

		driver, source, cred, create := tc.driverName, tc.source, "user:password", create

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

func TestClientCreateDir(t *testing.T) {
	runOnAllDrivers(t, func(tc testContext) {
		for i := 0; i < 10; i++ {
			port := "9998"

			user := "user"
			password := "password"

			driver, source, cred, create, addr := tc.driverName, tc.source, user+":"+password, create, ":"+port

			handler, e := createServer(&driver, &source, &cred, &create)
			assert.Nil(t, e)

			ctx, cancel := context.WithCancel(context.Background())
			srv := runServer(&addr, handler)

			uri := "http://localhost:" + port

			c := gowebdav.NewClient(uri, user, password)
			_ = c.Connect() // performs authorization
			err := c.Mkdir("folder", 0644)
			assert.Nil(t, err, "Got error %v", err)

			srv.Shutdown(ctx)
			cancel()
		}
	})
}
