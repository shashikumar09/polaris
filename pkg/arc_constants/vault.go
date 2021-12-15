package arc_constants


import (
	"fmt"	
	"encoding/json"
	"io/ioutil"
	"net/http"
	"github.com/ubccr/kerby/khttp"
)

type  VaultResponse struct {
	Data struct {
		ConnectionProperties struct {
			ApiKey string 	`json:"api_key"`
			AppKey string `json:"app_key"`
			} `json:"connection_properties"`
		} `json:"data"`
	}

func doHttpGet(uri string) (*http.Response, error) {
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	t := &khttp.Transport{}
	c := &http.Client{Transport: t}
	return c.Do(req)
}

func GetDDSecretsFromVault() VaultResponse {
	resp, err := doHttpGet("http://vault.ia55.net/v1/secret/slackbots/platformsre-bots/datadog/secrets")
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	var res VaultResponse
	json.Unmarshal(body, &res)
	return res
}

