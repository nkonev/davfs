package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	test "net/http/httptest"
	"testing"
)

const (
	driver = "memory"
	source = "."
	cred = ""
	create = false
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

func TestGetCredFailed(t *testing.T) {

	driver, source, cred, create := driver, source, "user:password", create

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
	assert.Equal(t, 401, resp.StatusCode)
}