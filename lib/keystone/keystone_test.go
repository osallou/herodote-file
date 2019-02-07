package keystone_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/osallou/herodote-file/lib/keystone"
)

const ksAuth = `{
	"token": {
		"project": {
			"domain": {
				"id": "Default",
				"name": "Default"
			},
			"id": "123",
			"name": "test"
		},
		"catalog": [
			{
				"endpoints": [{
					"url": "http://localhost/",
					"interface": "public"
				}],
				"type": "object-store"
			}
		]
	}
}`

func TestKeystoneAuth(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// fmt.Printf("%v", req.Header)
		fmt.Printf("Requested url %s\n", req.RequestURI)
		if req.RequestURI == "/v3/auth/tokens" {
			fmt.Printf("Return auth: %s\n", ksAuth)
			res.Header().Set("X-Subject-Token", "XYZ")
			res.WriteHeader(201)
			res.Write([]byte(ksAuth))
			return
		}
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	ksAuth := keystone.KeystoneAuth{}
	fmt.Printf("Server: %s\n", testServer.URL)
	ksAuth.OsAuthURL = testServer.URL + "/v3"
	ksAuth.OsUserDomainName = "Default"
	ksAuth.OsUserDomainID = "Default"
	ksAuth.OsProjectDomainName = "Default"
	ksAuth.OsProjectDomainID = "Default"
	ksAuth.OsProjectName = "test"
	ksAuth.OsUserName = "test"
	ksAuth.OsPassword = "XXXX"
	token, endpoint := keystone.Auth(ksAuth)
	if token != "XYZ" || endpoint != "http://localhost/AUTH_123" {
		t.Error(fmt.Sprintf("failure: %s, %s", token, endpoint))
	}

}
