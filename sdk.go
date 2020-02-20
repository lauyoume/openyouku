package openyouku

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/astaxie/beego"
)

type SDK struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	User         string
	Password     string
}

func (sdk *SDK) init() {
}

func (sdk *SDK) GetBytes(action string, params map[string]string) ([]byte, error) {

	sysParam := &SysParams{}
	sysParam.Action = action
	sysParam.ClientID = sdk.ClientID
	sysParam.Format = "json"
	sysParam.Timestamp = fmt.Sprint(time.Now().Unix())
	sysParam.Version = "3.0"
	if sdk.AccessToken != "" {
		sysParam.AccessToken = sdk.AccessToken
	}

	signParam := sysParam.SignParm(sdk.ClientSecret, params)

	jsonData, _ := json.Marshal(signParam)

	values := make(url.Values)

	for k, v := range params {
		values.Set(k, v)
	}
	uri := fmt.Sprintf("opensysparams=%s&%s", jsonData, values.Encode())
	url := "https://openapi.youku.com/router/rest.json?" + uri
	beego.Debug("url=>", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	beego.Debug("respbody=>", string(body))
	base := Response{}
	if err := json.Unmarshal(body, &base); err != nil {
		return nil, err
	}
	if base.Error.ErrorCode < 0 {
		return nil, errors.New(base.Error.ErrorMsg)
	}
	if base.Error.Code < 0 {
		return nil, errors.New(base.Error.ErrorMsg)
	}
	return body, err
}

func (sdk *SDK) Get(action string, params map[string]string) (*Response, error) {
	allBody, err := sdk.GetBytes(action, params)
	fmt.Println(string(allBody))
	response := &Response{}
	err = json.Unmarshal(allBody, response)
	return response, err
}

func (sdk *SDK) Post(action string, params map[string]interface{}) *Response {
	return nil
}

func (sdk *SDK) PostBytes(action string, params map[string]string) ([]byte, error) {

	sysParam := &SysParams{}
	sysParam.Action = action
	sysParam.ClientID = sdk.ClientID
	sysParam.Format = "json"
	sysParam.Timestamp = fmt.Sprint(time.Now().Unix())
	sysParam.Version = "3.0"
	if sdk.AccessToken != "" {
		sysParam.AccessToken = sdk.AccessToken
	}

	signParam := sysParam.SignParm(sdk.ClientSecret, params)
	jsonData, _ := json.Marshal(signParam)
	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}

	values.Set("opensysparams", string(jsonData))
	url := "https://openapi.youku.com/router/rest.json"
	beego.Debug("url=>", url)
	beego.Debug("params=>", values.Encode())
	resp, err := http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	beego.Debug("respbody=>", string(body))
	base := Response{}
	if err := json.Unmarshal(body, &base); err != nil {
		return nil, err
	}
	if base.Error.ErrorCode < 0 {
		return nil, errors.New(base.Error.ErrorMsg)
	}
	if base.Error.Code < 0 {
		return nil, errors.New(base.Error.ErrorMsg)
	}
	return body, err
}

func (sdk *SDK) GetUploader(name string, content []byte) *Uploader {
	return NewUploader(sdk, name, content)
}

func (sdk *SDK) GetUploaderWithFile(name string) (*Uploader, error) {
	return NewUploaderWithFile(sdk, name)
}

func (sdk *SDK) GetToken(code string) (*TokenResponse, error) {
	allBody, err := sdk.GetBytes(
		"youku.user.authorize.token.get",
		map[string]string{"code": code},
	)
	fmt.Println(string(allBody))
	response := &TokenResponse{}
	err = json.Unmarshal(allBody, response)
	return response, err
}

func (sdk *SDK) RefreshToken(token string) (*TokenResponse, error) {
	allBody, err := sdk.GetBytes(
		"youku.user.authorize.token.refresh",
		map[string]string{"refreshToken": token},
	)
	fmt.Println(string(allBody))
	response := &TokenResponse{}
	err = json.Unmarshal(allBody, response)
	return response, err
}
