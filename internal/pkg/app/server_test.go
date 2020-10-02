package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ziyasal/distroxy/pkg/distrox"
)

func TestServerPut_Get_Delete_Get(t *testing.T) {
	cache, err := distrox.NewCache()
	assert.Nil(t, err)

	srv := NewServer("http://unused.host", cache, WithMode("debug"))
	ts := httptest.NewServer(srv.newRouter())
	defer ts.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	key := "my-key"
	url := fmt.Sprintf("%s/v1/kv/%s", ts.URL, key)

	// PUT
	var want = []byte("\\x68\\x65\\x6C\\x6C\\x6F\\x20\\x77\\x6F\\x72\\x6C\\x64\\x21") // hello world!
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(want))

	resp, err := client.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, resp.Header.Get("Location"), fmt.Sprintf("%s/%s", cachePath, key))
	bodyRead, _ := ioutil.ReadAll(resp.Body)
	assert.Empty(t, bodyRead)

	// GET
	req, err = http.NewRequest("GET", url, nil)
	resp, err = client.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()

	got, _ := ioutil.ReadAll(resp.Body)
	assert.True(t, bytes.Equal(want, got))

	// DELETE and GET
	req, err = http.NewRequest("DELETE", url, nil)
	resp, err = client.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	req, err = http.NewRequest("GET", url, nil)
	resp, err = client.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
