package keystone

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	logs "github.com/osallou/hero-file/lib/log"
)

var logger = logs.GetLogger("hero.keystone")

// KeystoneAuth contains elements to get a keystone token
type KeystoneAuth struct {
	OsAuthURL           string
	OsUserDomainName    string
	OsUserDomainID      string
	OsProjectDomainName string
	OsProjectDomainID   string
	OsProjectName       string
	OsUserName          string
	OsPassword          string
}

type user struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Domain   dom    `json:"domain"`
}

type pass struct {
	User user `json:"user"`
}

type ident struct {
	Methods  []string `json:"methods"`
	Password pass     `json:"password"`
}

type dom struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type proj struct {
	Domain dom    `json:"domain"`
	Name   string `json:"name"`
}

type scope struct {
	Project proj `json:"project"`
}

type auth struct {
	Identity ident `json:"identity"`
	Scope    scope `json:"scope"`
}

type keystoneauth struct {
	Auth auth `json:"auth"`
}

// Auth gets a token from keystone
func Auth(ksAuth KeystoneAuth) (token string, endpointUrl string) {
	//server string, userDomain string, projectDomain string, project string, login string, password string) (token string) {
	client := &http.Client{}

	data := keystoneauth{
		Auth: auth{
			Identity: ident{
				Methods: []string{"password"},
				Password: pass{
					User: user{
						Name:     ksAuth.OsUserName,
						Password: ksAuth.OsPassword,
						Domain: dom{
							Name: ksAuth.OsUserDomainName,
							ID:   ksAuth.OsProjectDomainID}}}},
			Scope: scope{
				Project: proj{
					Domain: dom{
						Name: ksAuth.OsProjectDomainName,
						ID:   ksAuth.OsProjectDomainID},
					Name: ksAuth.OsProjectName}}}}

	jsonData, _ := json.Marshal(data)
	url := []string{ksAuth.OsAuthURL, "auth/tokens"}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	req, _ := http.NewRequest("POST", strings.Join(url, "/"), bytes.NewReader(jsonData))
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", ksAuth.OsAuthURL)
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		logger.Errorf("Error: %s\n", resp.Status)
		return "", ""
	}
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	errJSON := json.Unmarshal(bodyBytes, &result)
	if errJSON != nil {
		logger.Errorf("Failed to decode keystone answer")
		return "", ""
	}
	tokenInfo := result["token"].(map[string]interface{})
	projectInfo := tokenInfo["project"].(map[string]interface{})
	projectID := projectInfo["id"].(string)

	catalog := tokenInfo["catalog"].([]interface{})
	for i := range catalog {
		endpoint := catalog[i].(map[string]interface{})
		if endpoint["type"] == "object-store" {
			endpoints := endpoint["endpoints"].([]interface{})
			for j := range endpoints {
				endpointDef := endpoints[j].(map[string]interface{})
				if endpointDef["interface"] == "public" {
					endpointUrl = endpointDef["url"].(string)
					break
				}
			}
			break
		}
	}

	urlFragments := strings.Split(endpointUrl, "AUTH_")
	endpointUrl = urlFragments[0] + "AUTH_" + projectID

	token = resp.Header.Get("X-Subject-Token")
	return token, endpointUrl
}
