package funnel

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/jonson/tsgrok/internal/util"
)

func (s *HttpServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	displayFunnels := make([]DisplayFunnel, 0, len(s.funnelRegistry.Funnels))
	for _, funnel := range s.funnelRegistry.Funnels {
		df := DisplayFunnel{
			ID:          funnel.HTTPFunnel.id,
			LocalTarget: funnel.LocalTarget(),
			RemoteURL:   funnel.RemoteTarget(),
		}
		displayFunnels = append(displayFunnels, df)
	}

	data := struct {
		Title       string
		ProgramName string
		ActiveNav   string
		Funnels     []DisplayFunnel
	}{
		Title:       "Request Inspector",
		ProgramName: util.ProgramName,
		ActiveNav:   "Inspect",
		Funnels:     displayFunnels,
	}

	err := s.embeddedTemplates.ExecuteTemplate(w, "inspector.html", data)
	if err != nil {
		s.logger.Printf("Error executing inspector template: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (s *HttpServer) handleFunnelInspect(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/inspect/")
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 1 && parts[0] != "" {
		s.serveFunnelRequestsPage(w, r, parts[0])
		return
	}

	if len(parts) == 3 && parts[0] != "" && parts[1] == "request" && parts[2] != "" {
		s.handleFunnelRequestDetailFragment(w, r, parts[0], parts[2])
		return
	}

	if len(parts) == 5 && parts[0] != "" && parts[1] == "request" && parts[2] != "" && parts[3] == "body" && parts[4] == "request" {
		s.handleFunnelRequestBodyFragment(w, r, parts[0], parts[2])
		return
	}

	if len(parts) == 5 && parts[0] != "" && parts[1] == "request" && parts[2] != "" && parts[3] == "body" && parts[4] == "response" {
		s.handleFunnelResponseBodyFragment(w, r, parts[0], parts[2])
		return
	}

	http.NotFound(w, r)
}

func (s *HttpServer) serveFunnelRequestsPage(w http.ResponseWriter, r *http.Request, funnelID string) {
	funnel, err := s.GetFunnelById(funnelID)
	if err != nil {
		if errors.Is(err, ErrFunnelNotFound) {
			http.Error(w, "Funnel not found", http.StatusNotFound)
		} else {
			s.logger.Printf("Error retrieving funnel %s: %v", funnelID, err)
			http.Error(w, "Error retrieving funnel", http.StatusInternalServerError)
		}
		return
	}

	var capturedRequests []CaptureRequestResponse
	if funnel.Requests != nil {
		funnel.Requests.mu.Lock()
		currentNode := funnel.Requests.Head
		for currentNode != nil {
			capturedRequests = append(capturedRequests, currentNode.Request)
			currentNode = currentNode.Next
		}
		funnel.Requests.mu.Unlock()
	}

	data := struct {
		ProgramName string
		ActiveNav   string
		Funnel      struct {
			ID          string
			DisplayName string
			LocalTarget string
			RemoteURL   string
		}
		Requests []struct {
			UUID              string
			Method            string
			MethodClass       string
			RequestPath       string
			RequestURLString  string
			StatusClass       string
			StatusCode        int
			FormattedDuration string
		}
	}{
		ProgramName: util.ProgramName,
		ActiveNav:   "Inspect",
		Funnel: struct {
			ID          string
			DisplayName string
			LocalTarget string
			RemoteURL   string
		}{
			ID:          funnel.ID(),
			DisplayName: funnelName(funnel),
			LocalTarget: funnel.LocalTarget(),
			RemoteURL:   funnel.RemoteTarget(),
		},
	}

	for _, req := range capturedRequests {
		statusClass := "default"
		if req.Response.StatusCode >= 200 && req.Response.StatusCode < 300 {
			statusClass = "2xx"
		} else if req.Response.StatusCode >= 300 && req.Response.StatusCode < 400 {
			statusClass = "3xx"
		} else if req.Response.StatusCode >= 400 && req.Response.StatusCode < 500 {
			statusClass = "4xx"
		} else if req.Response.StatusCode >= 500 {
			statusClass = "5xx"
		}

		var requestPath string
		parsedReqURL, err := url.Parse(req.Request.URL)
		if err != nil {
			s.logger.Printf("Error parsing request URL '%s' in serveFunnelRequestsPage: %v", req.Request.URL, err)
			requestPath = req.Request.URL
		} else {
			requestPath = parsedReqURL.Path
			if requestPath == "" {
				requestPath = "/"
			}
		}

		data.Requests = append(data.Requests, struct {
			UUID              string
			Method            string
			MethodClass       string
			RequestPath       string
			RequestURLString  string
			StatusClass       string
			StatusCode        int
			FormattedDuration string
		}{
			UUID:              req.ID,
			Method:            req.Request.Method,
			MethodClass:       "method " + strings.ToLower(req.Request.Method),
			RequestPath:       requestPath,
			RequestURLString:  req.Request.URL,
			StatusClass:       statusClass,
			StatusCode:        req.Response.StatusCode,
			FormattedDuration: req.Duration.String(),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = s.embeddedTemplates.ExecuteTemplate(w, "funnel_requests.html", data)
	if err != nil {
		s.logger.Printf("Error executing funnel_requests template for funnel %s: %v", funnelID, err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (s *HttpServer) handleFunnelRequestDetailFragment(w http.ResponseWriter, r *http.Request, funnelID string, requestID string) {
	funnel, err := s.GetFunnelById(funnelID)
	if err != nil {
		if errors.Is(err, ErrFunnelNotFound) {
			http.Error(w, "Funnel not found", http.StatusNotFound)
		} else {
			s.logger.Printf("Error retrieving funnel %s: %v", funnelID, err)
			http.Error(w, "Error retrieving funnel", http.StatusInternalServerError)
		}
		return
	}

	capturedRequest := findRequestInList(funnel.Requests, requestID)

	if capturedRequest == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	var requestPath string
	var queryParams []QueryParamEntry
	parsedURL, err := url.Parse(capturedRequest.Request.URL)
	if err != nil {
		s.logger.Printf("Error parsing request URL string '%s': %v", capturedRequest.Request.URL, err)
		requestPath = capturedRequest.Request.URL
	} else {
		requestPath = parsedURL.Path
		if requestPath == "" {
			requestPath = "/"
		}
		for name, values := range parsedURL.Query() {
			for _, value := range values {
				queryParams = append(queryParams, QueryParamEntry{Name: name, Value: value})
			}
		}
	}

	details := ClientRequestDetails{
		FunnelID:     funnelID,
		UUID:         capturedRequest.ID,
		Path:         requestPath,
		Method:       capturedRequest.Request.Method,
		Status:       capturedRequest.Response.StatusCode,
		Duration:     capturedRequest.Duration.String(),
		Time:         capturedRequest.Timestamp.Format("2006-01-02 15:04:05"),
		ClientIP:     "N/A",
		RequestBody:  string(capturedRequest.Request.Body),
		ResponseBody: string(capturedRequest.Response.Body),
		QueryParams:  queryParams,
	}

	if xff, ok := capturedRequest.Request.Headers["X-Forwarded-For"]; ok && xff != "" {
		details.ClientIP = strings.Split(xff, ",")[0]
	} else if xri, ok := capturedRequest.Request.Headers["X-Real-Ip"]; ok && xri != "" {
		details.ClientIP = xri
	}

	requestHeaderNames := make([]string, 0, len(capturedRequest.Request.Headers))
	for name := range capturedRequest.Request.Headers {
		requestHeaderNames = append(requestHeaderNames, name)
	}
	sort.Strings(requestHeaderNames)
	for _, name := range requestHeaderNames {
		details.RequestHeaders = append(details.RequestHeaders, HeaderEntry{Name: name, Value: capturedRequest.Request.Headers[name]})
	}

	responseHeaderNames := make([]string, 0, len(capturedRequest.Response.Headers))
	for name := range capturedRequest.Response.Headers {
		responseHeaderNames = append(responseHeaderNames, name)
	}
	sort.Strings(responseHeaderNames)
	for _, name := range responseHeaderNames {
		details.ResponseHeaders = append(details.ResponseHeaders, HeaderEntry{Name: name, Value: capturedRequest.Response.Headers[name]})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = s.embeddedTemplates.ExecuteTemplate(w, "_request_detail_content.html", details)
	if err != nil {
		s.logger.Printf("Error executing request detail fragment template: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (s *HttpServer) handleFunnelRequestBodyFragment(w http.ResponseWriter, r *http.Request, funnelID string, requestID string) {
	s.serveRequestOrResponseBody(w, r, funnelID, requestID, true)
}

func (s *HttpServer) handleFunnelResponseBodyFragment(w http.ResponseWriter, r *http.Request, funnelID string, requestID string) {
	s.serveRequestOrResponseBody(w, r, funnelID, requestID, false)
}

func (s *HttpServer) serveRequestOrResponseBody(w http.ResponseWriter, r *http.Request, funnelID string, requestID string, isRequest bool) {
	funnel, err := s.GetFunnelById(funnelID)
	if err != nil {
		if errors.Is(err, ErrFunnelNotFound) {
			http.Error(w, "Funnel not found", http.StatusNotFound)
		} else {
			s.logger.Printf("Error retrieving funnel %s: %v", funnelID, err)
			http.Error(w, "Error retrieving funnel", http.StatusInternalServerError)
		}
		return
	}

	capturedRequest := findRequestInList(funnel.Requests, requestID)

	if capturedRequest == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	var bodyStr string
	if isRequest {
		bodyStr = string(capturedRequest.Request.Body)
	} else {
		bodyStr = string(capturedRequest.Response.Body)
	}

	data := struct {
		Body string
	}{
		Body: bodyStr,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = s.embeddedTemplates.ExecuteTemplate(w, "_body_content.html", data)
	if err != nil {
		s.logger.Printf("Error executing body content template: %v", err)
		http.Error(w, "Failed to render body content", http.StatusInternalServerError)
	}
}
