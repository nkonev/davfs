package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/studio-b12/gowebdav"
	"io/ioutil"
	test "net/http/httptest"
	"strings"
	"testing"
)

const (
	driver = "memory"
	source = "."
	cred   = ""
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

func TestGet(t *testing.T) {

	driver, source, cred, create := driver, source, cred, create

	handler, e := createServer(&driver, &source, &cred, &create)
	assert.Nil(t, e)

	req := test.NewRequest("GET", "/", nil)
	w := test.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()

	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Content-Type"))
	fmt.Println(string(body))
	assert.Equal(t, 405, resp.StatusCode)
}

func TestGetCredFailedWithoutCredentials(t *testing.T) {

	driver, source, cred, create := driver, source, "user:password", create

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
}

func TestGetCredFailedBadCredentials(t *testing.T) {

	driver, source, cred, create := driver, source, "user:password2", create

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
}

func TestGetCredSuccess(t *testing.T) {

	driver, source, cred, create := driver, source, "user:password", create

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
}

func TestClientCreateDir(t *testing.T) {
	//for i := 0; i < 100; i++{
	user := "user"
	password := "password"

	driver, source, cred, create, addr := driver, source, user+":"+password, create, ":9998"

	handler, e := createServer(&driver, &source, &cred, &create)
	assert.Nil(t, e)

	ctx, cancel := context.WithCancel(context.Background())
	srv := runServer(&addr, handler)
	defer srv.Shutdown(ctx)
	defer cancel()

	uri := "http://localhost:9998"

	c := gowebdav.NewClient(uri, user, password)
	_ = c.Connect() // performs authorization
	err := c.Mkdir("folder", 0644)
	assert.Nil(t, err, "Got error %v", err)

	//}
}
