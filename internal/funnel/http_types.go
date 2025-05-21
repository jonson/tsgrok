package funnel

import "errors"

// Error variables for common failure modes
var (
	ErrInvalidFunnelPath = errors.New("invalid path format for funnel request")
	ErrFunnelNotFound    = errors.New("funnel not found")
	ErrFunnelNotReady    = errors.New("funnel has no local target configured")
	ErrTargetURLParse    = errors.New("failed to parse funnel target URL")
)

// DisplayFunnel is used for displaying funnel information in the inspector.
type DisplayFunnel struct {
	ID          string
	LocalTarget string
	RemoteURL   string
}

// HeaderEntry is used for displaying request/response headers.
type HeaderEntry struct {
	Name  string
	Value string
}

// QueryParamEntry is used for displaying URL query parameters.
type QueryParamEntry struct {
	Name  string
	Value string
}

// ClientRequestDetails is the structure passed to the _request_detail_content.html template.
type ClientRequestDetails struct {
	FunnelID        string
	UUID            string
	Path            string
	Method          string
	Status          int
	Duration        string
	Time            string
	ClientIP        string
	RequestHeaders  []HeaderEntry
	ResponseHeaders []HeaderEntry
	RequestBody     string
	ResponseBody    string
	QueryParams     []QueryParamEntry
}

// FunnelIdAndRest holds the extracted funnel ID and the rest of the path.
type FunnelIdAndRest struct {
	id   string
	rest string
}
