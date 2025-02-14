package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arnavk2001/QuACK/quack"
)

type ResponseChecker struct {
	StatusCode  int
	FilePath    string
	ContentType string
	Close       bool
}

func findrequests(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting cwd: %v\n", err.Error())
	}
	basedir := path.Dir(path.Dir(cwd))
	requestsdir := path.Join(basedir, "tests", "requests")

	return requestsdir
}

var usehttpd = flag.String("usehttpd", "quack", "Which httpd server to use? ('quack' or 'go')")

func findhtdocs(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting cwd: %v\n", err.Error())
	}
	basedir := path.Dir(path.Dir(cwd))
	htdocsdir := path.Join(basedir, "tests", "htdocs1")

	return htdocsdir
}

func launchhttpd(t *testing.T) {
	switch *usehttpd {
	case "quack":
		launchquack(t)
	case "go":
		launchgohttpd(t)
	default:
		t.Fatalf("Invalid server type %v (must be 'quack' or 'go')", *usehttpd)
	}
}

func launchgohttpd(t *testing.T) {
	htdocs := findhtdocs(t)
	s := &http.Server{
		Addr:    ":8080",
		Handler: http.FileServer(http.Dir(htdocs)),
	}
	go s.ListenAndServe()
}

func launchquack(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(cwd)
	t.Log(cwd)
	virtualHosts := quack.ParseVHConfigFile("../../virtual_hosts.yaml", "../../docroot_dirs")
	s := &quack.Server{
		Addr:         ":8080",
		VirtualHosts: virtualHosts,
	}
	go s.ListenAndServe()
}

func TestGoFetch1(t *testing.T) {
	launchhttpd(t)

	req := fmt.Sprint("GET / HTTP/1.1\r\n"+
		"Host: website1\r\n",
		"Connection: close\r\n",
		"User-Agent: gotest\r\n",
		"\r\n")

	respbytes, _, err := quack.Fetch("localhost", "8080", []byte(req))
	if err != nil {
		t.Fatalf("Error fetching request: %v\n", err.Error())
	}

	respreader := bufio.NewReader(bytes.NewReader(respbytes))
	resp, err := http.ReadResponse(respreader, nil)
	if err != nil {
		t.Fatalf("got an error parsing the response: %v\n", err.Error())
	}

	if resp.Proto != "HTTP/1.1" {
		t.Fatalf("Expected HTTP/1.1 but got a version: %v\n", resp.Proto)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected response code of 200 but got: %v\n", resp.StatusCode)
	}

	if resp.ContentLength != 377 {
		t.Fatalf("Expected content length of 377 but got: %v\n", resp.ContentLength)
	}
}

func TestGoFetch2(t *testing.T) {
	launchhttpd(t)

	req := fmt.Sprint("GET / HTTP/1.1\r\n",
		"Host: website1\r\n",
		"User-Agent: gotest\r\n",
		"\r\n",
		"GET /notfound.html HTTP/1.1\r\n",
		"Host: website1\r\n",
		"User-Agent: gotest\r\n",
		"Connection: close\r\n",
		"\r\n",
	)

	respbytes, _, err := quack.Fetch("localhost", "8080", []byte(req))
	if err != nil {
		t.Fatalf("Error fetching request: %v\n", err.Error())
	}
	respreader := bufio.NewReader(bytes.NewReader(respbytes))

	// response 1
	resp, err := http.ReadResponse(respreader, nil)
	if err != nil {
		t.Fatalf("got an error parsing the response: %v\n", err.Error())
	}

	if resp.Proto != "HTTP/1.1" {
		t.Fatalf("Expected HTTP/1.1 but got a version: %v\n", resp.Proto)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected response code of 200 but got: %v\n", resp.StatusCode)
	}

	if resp.ContentLength != 377 {
		t.Fatalf("Expected content length of 377 but got: %v\n", resp.ContentLength)
	}

	indexbytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v\n", err.Error())
	}

	if len(indexbytes) != int(resp.ContentLength) {
		t.Fatalf("Response body of length %v was not equal to content length of %v\n",
			len(indexbytes), int(resp.ContentLength))
	}

	resp.Body.Close()

	// response 2
	resp, err = http.ReadResponse(respreader, nil)
	if err != nil {
		t.Fatalf("got an error parsing the response: %v\n", err.Error())
	}

	if resp.Proto != "HTTP/1.1" {
		t.Fatalf("Expected HTTP/1.1 but got a version: %v\n", resp.Proto)
	}

	if resp.StatusCode != 404 {
		t.Fatalf("Expected response code of 404 but got: %v\n", resp.StatusCode)
	}

	resp.Body.Close()
}

func TestGoFetch3(t *testing.T) {
	launchhttpd(t)

	req := fmt.Sprint("foobar\r\n"+
		"Host: website1\r\n",
		"Connection: close\r\n",
		"User-Agent: gotest\r\n",
		"\r\n")

	respbytes, _, err := quack.Fetch("localhost", "8080", []byte(req))
	if err != nil {
		t.Fatalf("Error fetching request: %v\n", err.Error())
	}

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(respbytes)), nil)
	if err != nil {
		t.Fatalf("got an error parsing the response: %v\n", err.Error())
	}

	if resp.Proto != "HTTP/1.1" {
		t.Fatalf("Expected HTTP/1.1 but got a version: %v\n", resp.Proto)
	}

	if resp.StatusCode != 400 {
		t.Fatalf("Expected response code of 400 but got: %v\n", resp.StatusCode)
	}
}

func TestAllFilesInHtdocs(t *testing.T) {
	launchhttpd(t)

	virtualHosts := quack.ParseVHConfigFile("../../virtual_hosts.yaml", "../../docroot_dirs")

	for hostname, docRoot := range virtualHosts {

		err := filepath.Walk(docRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				t.Fatal(err.Error())
			}

			if info.IsDir() {
				return nil
			}

			testfile := strings.TrimPrefix(path, docRoot)

			t.Run(testfile, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("recovered from panic: %v", r)
					}
				}()

				req := fmt.Sprintf("GET %s HTTP/1.1\r\n"+
					"Host: "+hostname+"\r\n"+
					"Connection: close\r\n"+
					"User-Agent: gotest\r\n"+
					"\r\n", testfile)

				respbytes, _, err := quack.Fetch("localhost", "8080", []byte(req))
				if err != nil {
					t.Fatalf("Error fetching request: %v\n", err.Error())
				}

				resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(respbytes)), nil)
				if err != nil {
					t.Fatalf("got an error parsing the response: %v\n", err.Error())
				}

				// Check the version
				if resp.Proto != "HTTP/1.1" {
					t.Fatalf("Expected HTTP/1.1 but got a version: %v\n", resp.Proto)
				}

				// Check the status code
				if resp.StatusCode != 200 {
					t.Fatalf("Expected response code of 200 but got: %v\n", resp.StatusCode)
				}

				// Check the Content-Type
				respcontenttype := resp.Header.Get("Content-Type")

				if respcontenttype == "" {
					t.Fatal("Response did not contain a Content-Type header")
				}

				origmimetype := mime.TypeByExtension(filepath.Ext(path))

				if !strings.HasPrefix(origmimetype, respcontenttype) {
					t.Fatalf("Expected Content-Type of %v but got %v instead\n", origmimetype, respcontenttype)
				}

				// Check the Content-Length
				if resp.ContentLength != info.Size() {
					t.Fatalf("Expected Content-Length of %v but got %v\n", info.Size(), resp.ContentLength)
				}

				// Verify the response body and the original file match
				origcontents, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Error reading input file: %v\n", err.Error())
				}

				respcontents, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Error reading response body: %v\n", err.Error())
				}
				resp.Body.Close()

				if !bytes.Equal(origcontents, respcontents) {
					t.Fatal("Response body does not equal original file contents")
				}
			})

			return nil
		})
		if err != nil {
			t.Fatal(err.Error())
		}
	}

}
