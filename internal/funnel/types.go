package funnel

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

type RequestListNode struct {
	Request CaptureRequestResponse
	Next    *RequestListNode
	Prev    *RequestListNode
}

type RequestList struct {
	Head      *RequestListNode
	Tail      *RequestListNode
	Length    int
	maxLength int
	mu        sync.Mutex
}

func (r *RequestList) Add(request CaptureRequestResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxLength > 0 && r.Length == r.maxLength {
		// remove the oldest request
		r.Tail = r.Tail.Prev
		if r.Tail == nil {
			r.Head = nil
		} else {
			r.Tail.Next = nil
		}
		r.Length--
	}

	node := &RequestListNode{Request: request}
	if r.Head == nil {
		// initial state, set head and tail to the same node
		r.Head = node
		r.Tail = node
	} else {
		r.Head.Prev = node
		node.Next = r.Head
		r.Head = node
	}
	r.Length++
}

type RequestResponse struct {
	Request  CaptureRequest
	Response CaptureResponse
	Duration time.Duration
}

type CaptureRequest struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string]string
}

type CaptureResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

type CaptureRequestResponse struct {
	ID        string
	FunnelID  string
	Timestamp time.Time
	Request   CaptureRequest
	Response  CaptureResponse
	Duration  time.Duration
}

func (r *CaptureRequestResponse) Method() string {
	return r.Request.Method
}

func (r *CaptureRequestResponse) URL() string {
	return r.Request.URL
}

func (r *CaptureRequestResponse) Path() string {
	// return the path of the URL
	parts, err := url.Parse(r.Request.URL)
	if err != nil {
		return ""
	}
	path := parts.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func (r *CaptureRequestResponse) StatusCode() int {
	return r.Response.StatusCode
}

func (r *CaptureRequestResponse) Type() string {
	// use the response content-type header to determine the type of the request
	contentType := r.Response.Headers["Content-Type"]
	if contentType == "" {
		return ""
	}

	// handle some explicit cases, json, html, xml, css, js, etc.
	if strings.HasPrefix(contentType, "application/json") {
		return "json"
	}
	if strings.HasPrefix(contentType, "text/html") {
		return "html"
	}
	if strings.HasPrefix(contentType, "text/xml") {
		return "xml"
	}
	if strings.HasPrefix(contentType, "text/css") {
		return "css"
	}
	if strings.HasPrefix(contentType, "text/javascript") {
		return "js"
	}
	if strings.HasPrefix(contentType, "text/plain") {
		return "txt"
	}
	// split the content type by "/" and take the first part
	parts := strings.Split(contentType, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (r *CaptureRequestResponse) RoundedDuration() string {
	if r.Duration.Seconds() >= 1 {
		// we want one decimal place
		return fmt.Sprintf("%.1fs", r.Duration.Seconds())
	}
	return fmt.Sprintf("%dms", r.Duration.Round(time.Millisecond).Milliseconds())
}
