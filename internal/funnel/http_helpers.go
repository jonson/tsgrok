package funnel

import (
	"strings"
)

// funnelName is a helper to get a display name for the funnel.
func funnelName(f Funnel) string {
	name := f.Name() // This method derives from RemoteTarget
	if name == "" {
		return f.ID() // Fallback to ID if name is empty
	}
	return name
}

// findRequestInList iterates through the funnel's request list to find a request by its ID.
func findRequestInList(requestList *RequestList, requestID string) *CaptureRequestResponse {
	if requestList == nil {
		return nil
	}
	requestList.mu.Lock()
	defer requestList.mu.Unlock()
	currentNode := requestList.Head
	for currentNode != nil {
		if currentNode.Request.ID == requestID {
			crCopy := currentNode.Request
			return &crCopy
		}
		currentNode = currentNode.Next
	}
	return nil
}

// singleJoiningSlash is a utility function for joining URL paths.
func singleJoiningSlash(a, b string) string {
	if a == "" && b == "" {
		return "/"
	}
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		if b == "" || a == "/" {
			return a + b
		}
		return a + "/" + b
	}
	return a + b
}

// extractFunnelIdAndRest extracts the funnel ID and the rest of the path from a URL path string.
func extractFunnelIdAndRest(pathAfterPrefix string) (FunnelIdAndRest, error) {
	if pathAfterPrefix == "" || pathAfterPrefix == "/" {
		return FunnelIdAndRest{}, ErrInvalidFunnelPath
	}

	parts := strings.SplitN(pathAfterPrefix, "/", 2)

	funnelId := ""
	rest := ""

	if len(parts) >= 1 {
		funnelId = parts[0]
	}
	if len(parts) == 2 {
		rest = parts[1]
	}

	if funnelId == "" {
		return FunnelIdAndRest{}, ErrInvalidFunnelPath
	}

	return FunnelIdAndRest{id: funnelId, rest: rest}, nil
}
