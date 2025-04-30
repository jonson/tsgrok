package funnel

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// Helper function to create a dummy request for testing
func newDummyRequest(id string) CaptureRequestResponse {
	return CaptureRequestResponse{ID: id, Timestamp: time.Now()}
}

func TestRequestList_Add(t *testing.T) {
	t.Run("Add to empty list", func(t *testing.T) {
		list := &RequestList{maxLength: 5}
		req1 := newDummyRequest("req1")
		list.Add(req1)

		if list.Length != 1 {
			t.Errorf("Expected length 1, got %d", list.Length)
		}
		if list.Head == nil || list.Head.Request.ID != "req1" {
			t.Errorf("Head node mismatch or nil")
		}
		if list.Tail == nil || list.Tail.Request.ID != "req1" {
			t.Errorf("Tail node mismatch or nil")
		}
		if list.Head != list.Tail {
			t.Errorf("Head and Tail should be the same for a single element list")
		}
		if list.Head.Prev != nil || list.Head.Next != nil {
			t.Errorf("Single node pointers should be nil")
		}
	})

	t.Run("Add multiple items below max length", func(t *testing.T) {
		list := &RequestList{maxLength: 5}
		req1 := newDummyRequest("req1")
		req2 := newDummyRequest("req2")
		req3 := newDummyRequest("req3")

		list.Add(req1) // List: req1
		list.Add(req2) // List: req2 -> req1
		list.Add(req3) // List: req3 -> req2 -> req1

		if list.Length != 3 {
			t.Errorf("Expected length 3, got %d", list.Length)
		}
		if list.Head == nil || list.Head.Request.ID != "req3" {
			t.Errorf("Expected Head ID 'req3', got %v", list.Head.Request.ID)
		}
		if list.Tail == nil || list.Tail.Request.ID != "req1" {
			t.Errorf("Expected Tail ID 'req1', got %v", list.Tail.Request.ID)
		}
		// Check links: req3 <-> req2 <-> req1
		if list.Head.Next == nil || list.Head.Next.Request.ID != "req2" {
			t.Errorf("Head.Next link incorrect")
		}
		if list.Head.Next.Prev != list.Head {
			t.Errorf("Head.Next.Prev link incorrect")
		}
		if list.Tail.Prev == nil || list.Tail.Prev.Request.ID != "req2" {
			t.Errorf("Tail.Prev link incorrect")
		}
		if list.Tail.Prev.Next != list.Tail {
			t.Errorf("Tail.Prev.Next link incorrect")
		}
	})

	t.Run("Add items exceeding max length", func(t *testing.T) {
		maxLength := 3
		list := &RequestList{maxLength: maxLength}
		reqs := []CaptureRequestResponse{
			newDummyRequest("req1"),
			newDummyRequest("req2"),
			newDummyRequest("req3"),
			newDummyRequest("req4"), // This should push out req1
			newDummyRequest("req5"), // This should push out req2
		}

		for _, req := range reqs {
			list.Add(req)
		}

		if list.Length != maxLength {
			t.Errorf("Expected length %d, got %d", maxLength, list.Length)
		}
		if list.Head == nil || list.Head.Request.ID != "req5" {
			t.Errorf("Expected Head ID 'req5', got %v", list.Head.Request.ID)
		}
		if list.Tail == nil || list.Tail.Request.ID != "req3" {
			t.Errorf("Expected Tail ID 'req3', got %v", list.Tail.Request.ID)
		}

		// Check remaining items: req5 <-> req4 <-> req3
		ids := []string{}
		curr := list.Head
		for curr != nil {
			ids = append(ids, curr.Request.ID)
			curr = curr.Next
		}
		expectedIDs := []string{"req5", "req4", "req3"}
		if !cmp.Equal(ids, expectedIDs) {
			t.Errorf("List contains wrong items. Got %v, want %v", ids, expectedIDs)
		}

		// Check links specifically
		if list.Head.Next == nil || list.Head.Next.Request.ID != "req4" || list.Head.Next.Prev != list.Head {
			t.Errorf("Link between req5 and req4 is broken")
		}
		if list.Tail.Prev == nil || list.Tail.Prev.Request.ID != "req4" || list.Tail.Prev.Next != list.Tail {
			t.Errorf("Link between req4 and req3 is broken")
		}
		if list.Head.Prev != nil {
			t.Errorf("Head.Prev should be nil")
		}
		if list.Tail.Next != nil {
			t.Errorf("Tail.Next should be nil")
		}
	})

	t.Run("Add with zero max length", func(t *testing.T) {
		list := &RequestList{maxLength: 0} // MaxLength 0 means unlimited
		req1 := newDummyRequest("req1")
		req2 := newDummyRequest("req2")

		list.Add(req1)
		list.Add(req2)

		if list.Length != 2 {
			t.Errorf("Expected length 2, got %d", list.Length)
		}
		if list.Head == nil || list.Head.Request.ID != "req2" {
			t.Errorf("Expected Head ID 'req2', got %v", list.Head.Request.ID)
		}
		if list.Tail == nil || list.Tail.Request.ID != "req1" {
			t.Errorf("Expected Tail ID 'req1', got %v", list.Tail.Request.ID)
		}
	})

	t.Run("Concurrency test (basic)", func(t *testing.T) {
		// This is a basic check, not exhaustive stress testing.
		// It verifies that concurrent Adds don't cause panics or obvious race conditions
		// due to the mutex.
		maxLength := 100
		list := &RequestList{maxLength: maxLength}
		numGoroutines := 10
		addsPerGoroutine := 20
		var wg sync.WaitGroup

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(gID int) {
				defer wg.Done()
				for j := 0; j < addsPerGoroutine; j++ {
					reqID := fmt.Sprintf("g%d_req%d", gID, j)
					list.Add(newDummyRequest(reqID))
				}
			}(i)
		}
		wg.Wait()

		// We expect the list length to be maxLength because total adds > maxLength
		expectedLen := maxLength
		totalAdds := numGoroutines * addsPerGoroutine
		if totalAdds < maxLength { // Adjust if total adds is less than max length
			expectedLen = totalAdds
		}

		if list.Length != expectedLen {
			t.Errorf("Expected length %d after concurrent adds, got %d", expectedLen, list.Length)
		}

		// Basic sanity check on list structure (no broken pointers)
		count := 0
		curr := list.Head
		for curr != nil {
			if curr.Next != nil && curr.Next.Prev != curr {
				t.Fatalf("Broken link detected: curr.Next.Prev != curr at item %d", count)
			}
			if curr.Prev != nil && curr.Prev.Next != curr {
				t.Fatalf("Broken link detected: curr.Prev.Next != curr at item %d", count)
			}
			if curr == list.Head && curr.Prev != nil {
				t.Fatalf("Head node has non-nil Prev pointer")
			}
			if curr == list.Tail && curr.Next != nil {
				t.Fatalf("Tail node has non-nil Next pointer")
			}
			curr = curr.Next
			count++
		}
		if count != list.Length {
			t.Errorf("Iterated count %d does not match list.Length %d", count, list.Length)
		}

	})
}

func TestCaptureRequestResponse_RoundedDuration(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"Zero duration", 0, "0ms"},
		{"Milliseconds", 123 * time.Millisecond, "123ms"},
		{"Half second", 500 * time.Millisecond, "500ms"},
		{"Just under 1 second", 999 * time.Millisecond, "999ms"},
		{"Exactly 1 second", 1 * time.Second, "1.0s"},
		{"1.2 seconds", 1200 * time.Millisecond, "1.2s"},
		{"1.23 seconds (rounds)", 1234 * time.Millisecond, "1.2s"}, // Should round based on fmt.Sprintf
		{"Long duration", 5*time.Minute + 30*time.Second + 456*time.Millisecond, "330.5s"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			crr := CaptureRequestResponse{Duration: tc.duration}
			if got := crr.RoundedDuration(); got != tc.expected {
				t.Errorf("Expected RoundedDuration() to be %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestCaptureRequestResponse_Type(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		expected    string
	}{
		{"No Content-Type", "", ""},
		{"JSON", "application/json", "json"},
		{"JSON with charset", "application/json; charset=utf-8", "json"},
		{"HTML", "text/html", "html"},
		{"XML", "text/xml", "xml"},
		{"CSS", "text/css", "css"},
		{"JavaScript", "text/javascript", "js"},
		{"Plain Text", "text/plain", "txt"},
		{"Image PNG", "image/png", "image"},
		{"Video MP4", "video/mp4", "video"},
		{"Application Octet Stream", "application/octet-stream", "application"},
		{"Weird format", "foo/bar", "foo"},
		{"Only main type", "audio", "audio"}, 
		{"Empty string after split", "/", ""}, // Invalid Content-Type
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			headers := make(map[string]string)
			if tc.contentType != "" {
				headers["Content-Type"] = tc.contentType
			}
			crr := CaptureRequestResponse{
				Response: CaptureResponse{Headers: headers},
			}
			if got := crr.Type(); got != tc.expected {
				t.Errorf("Expected Type() to be %q for Content-Type %q, got %q", tc.expected, tc.contentType, got)
			}
		})
	}
}
