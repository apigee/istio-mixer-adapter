package analytics

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"github.com/apigee/istio-mixer-adapter/apigee/authtest"
	adaptertest "istio.io/istio/mixer/pkg/adapter/test"
)

type testRecordPush struct {
	records []Record
	dir     string
}

type fakeServer struct {
	records map[string][]testRecordPush
	srv     *httptest.Server
}

func newFakeServer(t *testing.T) *fakeServer {
	fs := &fakeServer{
		records: map[string][]testRecordPush{},
	}
	fs.srv = httptest.NewServer(fs.handler(t))
	return fs
}

func (fs *fakeServer) handler(t *testing.T) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/analytics", func(w http.ResponseWriter, r *http.Request) {
		// Give them a signed URL. Include the file path they picked so that we can
		// confirm they are sending the right one.
		url := "%s/signed-url-1234?relative_file_path=%s&tenant=%s"
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url": fmt.Sprintf(url, fs.srv.URL, r.FormValue("relative_file_path"), r.FormValue("tenant")),
		})
	})
	m.HandleFunc("/signed-url-1234", func(w http.ResponseWriter, r *http.Request) {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("Error on gzip.NewReader: %s", err)
		}
		defer gz.Close()
		defer r.Body.Close()

		rec := []Record{}
		if err := json.NewDecoder(gz).Decode(&rec); err != nil {
			t.Fatalf("Error decoding JSON sent to signed URL: %s", err)
		}
		tenant := r.FormValue("tenant")
		fp := r.FormValue("relative_file_path")
		fs.records[tenant] = append(fs.records[tenant], testRecordPush{
			records: rec,
			dir:     path.Dir(fp),
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return m
}

func (fs *fakeServer) Close()                               { fs.srv.Close() }
func (fs *fakeServer) Records() map[string][]testRecordPush { return fs.records }
func (fs *fakeServer) URL() string                          { return fs.srv.URL }

func TestPushAnalytics(t *testing.T) {
	fs := newFakeServer(t)
	defer fs.Close()

	m := newUAPBackend().(*uapBackend)

	fp1 := "hi~test"
	fp2 := "otherorg~test"

	// This timestamp is roughly 11:30 MST on Mar. 16, 2018.
	ts := int64(1521221450)
	m.now = func() time.Time { return time.Unix(ts, 0) }

	wantRecords := map[string][]testRecordPush{
		fp1: {
			{
				records: []Record{{APIProxy: "proxy"}, {APIProduct: "product"}},
				dir:     fmt.Sprintf("date=2018-03-16/time=%d-%d", ts, ts),
			},
		},
		fp2: {
			{
				records: []Record{{RequestURI: "request URI"}},
				dir:     fmt.Sprintf("date=2018-03-16/time=%d-%d", ts, ts),
			},
		},
	}

	env := adaptertest.NewEnv(t)
	m.collectionInterval = 50 * time.Millisecond
	m.Start(env)
	defer m.Close()

	tc := authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("hi")
	tc.SetEnvironment("test")
	ctx := &auth.Context{Context: tc}

	// Send them in batches to ensure we group them all together.
	if err := m.SendRecords(ctx, wantRecords[fp1][0].records[:1]); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}
	if err := m.SendRecords(ctx, wantRecords[fp1][0].records[1:]); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Send one more with a different org on a "different" day>
	tc = authtest.NewContext(fs.URL(), env)
	tc.SetOrganization("otherorg")
	tc.SetEnvironment("test")
	ctx = &auth.Context{Context: tc}
	if err := m.SendRecords(ctx, wantRecords[fp2][0].records); err != nil {
		t.Errorf("Error on SendRecords(): %s", err)
	}

	// Records are sent async, so we should not have sent any yet.
	if len(fs.Records()) > 0 {
		t.Errorf("Got %d records sent, want 0: %v", len(fs.Records()), fs.Records())
	}

	time.Sleep(100 * time.Millisecond)

	if !reflect.DeepEqual(fs.Records(), wantRecords) {
		t.Errorf("got records %v, want records %v", fs.Records(), wantRecords)
	}
}
