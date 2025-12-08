package request

import (
	"net/url"
	"testing"
)

type Params struct {
	Arg1 int    `param:"arg1"`
	Arg2 string `param:"arg2"`
	Arg3 bool   `param:"arg3"`
}

func TestMarshallParams(t *testing.T) {

	tUrl, err := url.Parse("example.com?arg1=1&arg2=hello_world&arg3=true")
	if err != nil {
		t.Fatalf("unable to parse url Query: %s", err.Error())
	}

	tUrl2, err := url.Parse("example.com?arg1=hello&arg")
	if err != nil {
		t.Fatalf("unable to parse url Query: %s", err.Error())
	}

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		urlValues url.Values
		dest      Params
		wantErr   bool
	}{
		{
			name:      "simple-url-no-error-expected",
			urlValues: tUrl.Query(),
			wantErr:   false,
		},
		{
			name:      "url-params-not-correct-type",
			urlValues: tUrl2.Query(),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := MarshallParams(tt.urlValues, &tt.dest)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("MarshallParams() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Log(tt.dest)
				t.Fatal("MarshallParams() succeeded unexpectedly")
			}
			t.Log(tt.dest)
		})
	}
}
