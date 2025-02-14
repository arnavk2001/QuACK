package quack

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const HTTP_Version = "HTTP/1.1"
const STATUS_OK = 200
const STATUS_BAD = 400
const STATUS_NOT_FOUND = 404

var statusText = map[int]string{
	STATUS_OK:        "OK",
	STATUS_BAD:       "Bad Request",
	STATUS_NOT_FOUND: "Not Found",
}

type Server struct {
	// Addr specifies the TCP address for the server to listen on,
	// in the form "host:port". It shall be passed to net.Listen()
	// during ListenAndServe().
	Addr string // e.g. ":0"

	// VirtualHosts contains a mapping from host name to the docRoot path
	// (i.e. the path to the directory to serve static files from) for
	// all virtual hosts that this server supports
	VirtualHosts map[string]string
}

// ListenAndServe listens on the TCP network address s.Addr and then
// handles requests on incoming connections.
func (s *Server) ListenAndServe() error {
	err := s.ValidateSetup()
	// Validate all docRoots
	if err != nil {
		return fmt.Errorf("error encountered in Setup")
	}

	// create a listen socket and spawn off goroutines per incoming client
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	//accept connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		fmt.Println("Accepted Connection from ", conn.RemoteAddr())
		//spawn a go routine
		go s.HandleConnection(conn)
	}

}

func (s *Server) ValidateSetup() error {
	//check if docroot exists
	for _, v := range s.VirtualHosts {

		f, err := os.Stat(v)
		if os.IsNotExist(err) {
			fmt.Println("docroot does not exist")
			return err
		}
		//check if it is a directory
		if !f.IsDir() {
			return fmt.Errorf("given location is not a directory")
		}
	}

	return nil
}

// Handle the connections
func (s *Server) HandleConnection(conn net.Conn) {
	br := bufio.NewReader(conn)
	defer conn.Close()

	for {

		//set timeout
		err := conn.SetReadDeadline(time.Now().Add(time.Second * 5))
		if err != nil {
			fmt.Println("Failed to set timer")

			return
		}

		res := &Response{
			Headers: make(map[string]string),
		}

		//Call Read Request and handle good and bad requests
		req, isRequestPartial, err := ReadRequest(br, s, res)

		res.Request = req

		if errors.Is(err, io.EOF) {
			log.Printf("Connection closed by %v", conn.RemoteAddr())
			return
		}
		// Handle Timeout
		if err, ok := err.(net.Error); ok && err.Timeout() {
			log.Printf("Connection to %v timed out", conn.RemoteAddr())
			if isRequestPartial {
				log.Printf("Handle bad request for partial request")
				res.HandleBadRequest()
				res.Write(conn)
			}
			return
		}

		if err != nil {
			log.Printf("Handle bad request for error: %v", err)
			res.HandleBadRequest()
			err = res.Write(conn)
			if err != nil {
				fmt.Println("write function did not work")
			}

			return
		}

		root, ok := s.VirtualHosts[req.Headers["Host"]]
		if !ok {
			fmt.Println("Host not found", req.Headers["Host"])
			res.HandleNotFound()
			res.Write(conn)
			if checkCloseConnection(req) {
				return
			}
			continue
		}

		// check if url starts with /
		if !strings.HasPrefix(req.URL, "/") {
			log.Printf("Handle bad request for error: %v", err)
			res.HandleBadRequest()
			err = res.Write(conn)
			if err != nil {
				fmt.Println("write function did not work")
			}

			return
		}

		if !validURL(root, req.URL, res) {
			fmt.Println("invalid URL", req.URL)
			res.HandleNotFound()
			res.Write(conn)
			if checkCloseConnection(req) {
				return
			}
			continue
		}

		fmt.Println("iufiehighf", res.FilePath)

		//if the req.URL does not exist in the docroot, return 404
		//if the req.URL exists in the docroot, return 200
		_, err = os.Stat(res.FilePath)
		if os.IsNotExist(err) {
			fmt.Println("docroot does not exist")
			res.HandleNotFound()
			res.Write(conn)
			if checkCloseConnection(req) {
				return
			}
			continue
		}

		// Handle good request
		log.Printf("Handle good request: %v", req)
		res.HandleGoodRequest()
		err = res.Write(conn)
		if err != nil {
			fmt.Println("write function did not work")
		}
		//check if req.Headers["Connection"] is present abd if it is and it is set to close, close the connection
		if checkCloseConnection(req) {
			return
		}

	}

}

func checkCloseConnection(req *Request) bool {
	if val, ok := req.Headers["Connection"]; ok && val == "close" {
		return true
	}
	return false
}

func (res *Response) HandleGoodRequest() {
	res.Proto = HTTP_Version
	res.StatusCode = STATUS_OK

}

func (res *Response) HandleBadRequest() {
	res.FilePath = ""
	res.Proto = HTTP_Version
	res.StatusCode = STATUS_BAD
	res.Headers["Connection"] = "close"
}

func (res *Response) HandleNotFound() {
	res.FilePath = ""
	res.Proto = HTTP_Version
	res.StatusCode = STATUS_NOT_FOUND
}

func ReadRequest(br *bufio.Reader, s *Server, res *Response) (req *Request, isRequestPartial bool, err error) {
	req = &Request{Headers: make(map[string]string)}
	line, err := ReadLine(br)
	if err != nil {
		fmt.Println("Error while reading the line", err)
		return nil, len(line) > 0, err
	}

	req.Method, req.URL, req.Proto, err = parseRequestFirstLine(line)
	if err != nil {
		return nil, true, badStringError("malformed start line", line)
	}

	if !validateMethod(req.Method) {
		return nil, true, badStringError("invalid method", req.Method)
	}

	if !validateProto(req.Proto) {
		return nil, true, badStringError("invalid proto", req.Proto)
	}

	for {
		line, err := ReadLine(br)
		if err != nil {
			return nil, true, err
		}
		if line == "" {
			// This marks header end
			break
		}

		//Use parseRequestLines function to read other lines of the request
		err = parseRequestLines(line, req)
		if err != nil {
			fmt.Println("error encountered in parsing request lines of the headers", err)
		}

		fmt.Println("Read line from request", line)
	}
	//check if req.Headers["Host"] is present
	if _, ok := req.Headers["Host"]; !ok {
		return nil, true, badStringError("Host header not present", "")
	}

	return req, true, nil

}

func parseRequestLines(line string, req *Request) error {
	str := strings.SplitN(line, ":", 2)

	if len(str) != 2 {
		return fmt.Errorf("key value pairs in the header are incorrect %v", line)
	}

	if containsWhitespace(str[0]) {
		return fmt.Errorf("key contains whitespace")
	}

	// Check if the key contanins only alphanumeric characters or hyphen
	for _, char := range str[0] {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '-' {
			return fmt.Errorf("key contains invalid characters")
		}
	}

	//canonicalize the key
	str[0] = CanonicalHeaderKey(str[0])
	req.Headers[str[0]] = strings.TrimLeftFunc(str[1], unicode.IsSpace)
	return nil
}

// Parses first line of the request and returns Method, URL and Proto of the reques
func parseRequestFirstLine(line string) (string, string, string, error) {
	fields := strings.SplitN(line, " ", 3)
	if len(fields) != 3 {
		return "", "", "", fmt.Errorf("could not parse the request line, got fields %v", fields)
	}
	return fields[0], fields[1], fields[2], nil
}

// validate method if it is GET
func validateMethod(method string) bool {
	return method == "GET"
}

// validate URL function cleans the URL of ../ and then appends the URL to the docroot to get the file path
func validURL(docRoot string, url string, res *Response) bool {
	//if the URL contains '/' at the end then append index.html to the URL
	if strings.HasSuffix(url, "/") {
		url = url + "index.html"
	}
	url = docRoot + url
	url = path.Clean(url)
	absolute_url, err := filepath.Abs(url)
	fmt.Println(url, absolute_url)
	if err != nil {
		fmt.Println("Error encountered in getting absolute path")
	}
	absolute_docRoot, err := filepath.Abs(docRoot)
	if err != nil {
		fmt.Println("Error encountered in getting absolute path")
	}
	if strings.HasPrefix(absolute_url, absolute_docRoot) {
		res.FilePath = absolute_url
		return true
	}

	return false

}

// validate Proto
func validateProto(proto string) bool {
	return proto == HTTP_Version
}

func badStringError(what, val string) error {
	return fmt.Errorf("%s %q", what, val)
}

func ReadLine(br *bufio.Reader) (string, error) {
	var line string
	for {
		s, err := br.ReadString('\n')
		line += s
		// Return the error
		if err != nil {
			return line, err
		}
		// Return the line when reaching line end
		if strings.HasSuffix(line, "\r\n") {
			// Striping the line end
			line = line[:len(line)-2]
			return line, nil
		}
	}
}

func containsWhitespace(s string) bool {
	for _, char := range s {
		if unicode.IsSpace(char) {
			return true
		}
	}
	return false
}

func (res *Response) Write(w io.Writer) error {
	bw := bufio.NewWriter(w)
	//make an array of strings
	var array []string

	statusLine := fmt.Sprintf("%v %v %v\r\n", res.Proto, res.StatusCode, statusText[res.StatusCode])
	fmt.Println("Status Line", res.Proto, res.StatusCode, statusText[res.StatusCode])
	if _, err := bw.WriteString(statusLine); err != nil {
		return err
	}
	for k, v := range res.Headers {
		headerLine := fmt.Sprintf("%v: %v\r\n", k, v)
		if _, err := bw.WriteString(headerLine); err != nil {
			return err
		}
	}

	//write date header
	Date := FormatTime(time.Now())
	DateHeader := fmt.Sprintf("Date: %v\r\n", Date)
	fmt.Println("Response is", res)

	array = append(array, DateHeader)

	//check if the connection header in the request in response is set to close
	if res.Request != nil && res.Request.Headers["Connection"] == "close" {
		connectionHeader := fmt.Sprintf("Connection: close\r\n")
		array = append(array, connectionHeader)
	}

	//if the response is 200, then write last modified, content type and content length headers
	if res.StatusCode == STATUS_OK {

		//write content type header
		contentType := MIMETypeByExtension(filepath.Ext(res.FilePath))
		contentTypeHeader := fmt.Sprintf("Content-Type: %v\r\n", contentType)
		array = append(array, contentTypeHeader)

		//write content length and lastModified header

		//write the file to the connection
		file, err := os.Open(res.FilePath)
		if err != nil {
			fmt.Println("Error encountered in opening the file")
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		lastModified := FormatTime(fileInfo.ModTime())
		if err != nil {
			fmt.Println("Error encountered in getting file info")
		}
		contentLengthHeader := fmt.Sprintf("Content-Length: %v\r\n", fileInfo.Size())
		lastModifiedHeader := fmt.Sprintf("Last-Modified: %v\r\n", lastModified)
		array = append(array, contentLengthHeader)
		array = append(array, lastModifiedHeader)

	}
	sort.Strings(array)

	for _, v := range array {
		if _, err := bw.WriteString(v); err != nil {
			return fmt.Errorf("error encountered in writing the headers")
		}
	}

	if _, err := bw.WriteString("\r\n"); err != nil {
		return err
	}

	if res.StatusCode == STATUS_OK {

		//write the file to the connection
		byts, _ := os.ReadFile(res.FilePath)
		_, err := bw.Write(byts)
		if err != nil {
			fmt.Println("Error encountered in copying the file")
		}

	}

	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}
