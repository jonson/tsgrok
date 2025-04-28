package funnel

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func Test_extractFunnelIdAndRest(t *testing.T) {
	tests := []struct {
		name            string
		pathAfterPrefix string
		want            FunnelIdAndRest
		wantErr         bool // Use this if the function is expected to return an error
	}{
		{
			name:            "Path with ID and rest",
			pathAfterPrefix: "funnel123/some/other/path",
			want:            FunnelIdAndRest{id: "funnel123", rest: "some/other/path"},
			wantErr:         false,
		},
		{
			name:            "Path with only ID",
			pathAfterPrefix: "funnel456",
			want:            FunnelIdAndRest{id: "funnel456", rest: ""},
			wantErr:         false,
		},
		{
			name:            "Path with ID and trailing slash",
			pathAfterPrefix: "funnel789/",
			want:            FunnelIdAndRest{id: "funnel789", rest: ""},
			wantErr:         false,
		},
		{
			name:            "Empty path",
			pathAfterPrefix: "",
			want:            FunnelIdAndRest{id: "", rest: ""},
			wantErr:         true,
		},
		{
			name:            "Path with only slash",
			pathAfterPrefix: "/",
			want:            FunnelIdAndRest{id: "", rest: ""},
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFunnelIdAndRest(tt.pathAfterPrefix)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractFunnelIdAndRest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(FunnelIdAndRest{})); diff != "" {
				t.Errorf("extractFunnelIdAndRest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_singleJoiningSlash(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{
			name: "Both non-empty, no slashes",
			a:    "path1",
			b:    "path2",
			want: "path1/path2",
		},
		{
			name: "a ends with slash, b starts with slash",
			a:    "path1/",
			b:    "/path2",
			want: "path1/path2",
		},
		{
			name: "a ends with slash, b no slash",
			a:    "path1/",
			b:    "path2",
			want: "path1/path2",
		},
		{
			name: "a no slash, b starts with slash",
			a:    "path1",
			b:    "/path2",
			want: "path1/path2",
		},
		{
			name: "a is empty",
			a:    "",
			b:    "path2",
			want: "/path2", // Should result in leading slash
		},
		{
			name: "b is empty",
			a:    "path1",
			b:    "",
			want: "path1", // Should not add trailing slash
		},
		{
			name: "a is empty, b starts with slash",
			a:    "",
			b:    "/path2",
			want: "/path2",
		},
		{
			name: "a ends with slash, b is empty",
			a:    "path1/",
			b:    "",
			want: "path1/", // Preserve trailing slash if b is empty
		},
		{
			name: "Both empty",
			a:    "",
			b:    "",
			want: "/",
		},
		{
			name: "a is slash",
			a:    "/",
			b:    "path2",
			want: "/path2",
		},
		{
			name: "b is slash",
			a:    "path1",
			b:    "/",
			want: "path1/",
		},
		{
			name: "Both are slashes",
			a:    "/",
			b:    "/",
			want: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := singleJoiningSlash(tt.a, tt.b); got != tt.want {
				t.Errorf("singleJoiningSlash(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
