package funnel

import (
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
