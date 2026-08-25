package process

import (
	"slices"
	"testing"
)

func TestHostPaths(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    []string
		wantErr bool
	}{
		{
			"a mounted overlay is addressed through its merged view",
			"1\n" + overlayMeta,
			[]string{testRoot + "/w1/merged/etc/app.conf"},
			false,
		},
		{
			"an unmounted overlay reads the upper dir, then the bundle",
			"0\n" + overlayMeta,
			[]string{testRoot + "/w1/upper/etc/app.conf", testRoot + "/w1/lower/etc/app.conf"},
			false,
		},
		{
			"a raw workload is addressed under its working directory",
			"0\n" + rawMeta,
			[]string{"/srv/app/etc/app.conf"},
			false,
		},
		{"a workload with no record is not found", "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := 0
			if tt.wantErr {
				code = notExistsCode
			}
			e := testEngine(t, &fakeRunner{respond: func(string) *result { return &result{Stdout: tt.stdout, Code: code} }})

			got, err := e.hostPaths(t.Context(), "w1", "/etc/app.conf")
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
