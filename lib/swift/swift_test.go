package swift_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	swift "github.com/osallou/herodote-file/lib/swift"
)

func swiftSimulator(res http.ResponseWriter, req *http.Request) {
	// fmt.Printf("%v", req.Header)
	files := make(map[string]string)
	files["/project/withManifest"] = ""
	files["/project_segments/withManifest/file.1"] = "a"
	files["/project_segments/withManifest/file.2"] = "b"
	files["/project_segments/withManifest/file.3"] = "c"
	files["/project/withoutManifest"] = "def"
	if req.Header.Get("X-Auth-Token") == "" {
		res.WriteHeader(401)
		res.Write([]byte("no token"))
		return
	}

	var val string = ""

	if req.Method == "HEAD" {
		fmt.Printf("Requested url %s\n", req.RequestURI)
		if req.RequestURI == "/project/withManifest" {
			res.Header().Set("X-Object-Manifest", "/project_segments/withManifest")
		}
	} else if req.Method == "GET" {
		var ok bool
		val, ok = files[req.RequestURI]
		if !ok {
			res.WriteHeader(404)
			res.Write([]byte("not found"))
			return
		}
	} else if req.Method == "PUT" {
		files[req.RequestURI] = "newfile"
		// not managing meta here (X-Object-Meta-)
	} else if req.Method == "POST" {
		// nothing to do, not managing meta here (X-Object-Meta-)
	}
	res.WriteHeader(200)
	res.Write([]byte(val))
}

func TestSwiftHead(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(swiftSimulator))
	defer func() { testServer.Close() }()
	options := swift.Options{}
	options.Bucket = "project"
	options.File = "withoutManifest"
	res := swift.Head("123", testServer.URL, options)
	if res != "" {
		t.Error("should have no manifest: " + res)
	}
	options.File = "withManifest"
	res = swift.Head("123", testServer.URL, options)
	if res != "/project_segments/withManifest" {
		t.Error("should have manifest: " + res)
	}
}

func TestSwiftStat(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// fmt.Printf("%v", req.Header)
		if req.Header.Get("X-Auth-Token") == "" {
			res.WriteHeader(401)
			res.Write([]byte("no token"))
		}
		if req.RequestURI == "//" {
			res.Header().Set("X-Account-Container-Count", "1")
		} else if req.RequestURI == "/project/" {
			res.Header().Set("X-Container-Object-Count", "1")
		} else {
			res.Header().Set("X-Object-Meta-test", "test")
		}
		fmt.Printf("Requested url %s\n", req.RequestURI)
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	fmt.Printf("Server: %s\n", testServer.URL)
	options := swift.Options{}
	options.Bucket = ""
	options.File = ""
	res, _ := swift.Show("123", testServer.URL, options)
	if _, ok := res["X-Account-Container-Count"]; !ok {
		t.Error("stat failed")
	}
	options.Bucket = "project"
	options.File = ""
	res, _ = swift.Show("123", testServer.URL, options)
	if _, ok := res["X-Container-Object-Count"]; !ok {
		t.Error("stat failed")
	}
	options.Bucket = "project"
	options.File = "myfile"
	res, _ = swift.Show("123", testServer.URL, options)
	if _, ok := res["X-Object-Meta-Test"]; !ok {
		t.Error("stat failed")
	}

}
