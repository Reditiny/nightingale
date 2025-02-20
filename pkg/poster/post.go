package poster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/conf"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/toolkits/pkg/logger"
)

type DataResponse[T any] struct {
	Dat T      `json:"dat"`
	Err string `json:"err"`
}

func GetByUrls[T any](ctx *ctx.Context, path string) (T, error) {
	addrs := ctx.CenterApi.Addrs
	if len(addrs) == 0 {
		var dat T
		return dat, fmt.Errorf("no center api addresses configured")
	}

	// 随机选择起始位置
	startIdx := rand.Intn(len(addrs))

	// 从随机位置开始遍历所有地址

	var dat T
	var err error
	for i := 0; i < len(addrs); i++ {
		idx := (startIdx + i) % len(addrs)
		url := fmt.Sprintf("%s%s", addrs[idx], path)

		dat, err = GetByUrl[T](url, ctx.CenterApi)
		if err != nil {
			logger.Warningf("failed to get data from center, url: %s, err: %v", url, err)
			continue
		}
		return dat, nil
	}

	return dat, fmt.Errorf("failed to get data from center, path= %s, addrs= %v err: %v", path, addrs, err)
}

func GetByUrl[T any](url string, cfg conf.CenterApi) (T, error) {
	var dat T

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return dat, fmt.Errorf("failed to create request: %w", err)
	}

	if len(cfg.BasicAuthUser) > 0 {
		req.SetBasicAuth(cfg.BasicAuthUser, cfg.BasicAuthPass)
	}

	if cfg.Timeout < 1 {
		cfg.Timeout = 5000
	}

	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
	}

	if UseProxy(url) {
		client.Transport = ProxyTransporter
	}

	resp, err := client.Do(req)
	if err != nil {
		return dat, fmt.Errorf("failed to fetch from url: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return dat, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return dat, fmt.Errorf("failed to read response body: %w", err)
	}

	var dataResp DataResponse[T]
	err = json.Unmarshal(body, &dataResp)
	if err != nil {
		return dat, fmt.Errorf("failed to decode:%s response: %w", string(body), err)
	}

	if dataResp.Err != "" {
		return dat, fmt.Errorf("error from server: %s", dataResp.Err)
	}

	logger.Debugf("get data from %s, data: %+v", url, dataResp.Dat)
	return dataResp.Dat, nil
}

func PostByUrls(ctx *ctx.Context, path string, v interface{}) error {
	addrs := ctx.CenterApi.Addrs
	if len(addrs) == 0 {
		return fmt.Errorf("submission of the POST request from the center has failed, "+
			"path= %s, v= %v, ctx.CenterApi.Addrs= %v", path, v, addrs)
	}

	// 随机选择起始位置
	startIdx := rand.Intn(len(addrs))

	// 从随机位置开始遍历所有地址
	for i := 0; i < len(addrs); i++ {
		idx := (startIdx + i) % len(addrs)
		url := fmt.Sprintf("%s%s", addrs[idx], path)

		_, err := PostByUrl[interface{}](url, ctx.CenterApi, v)
		if err != nil {
			logger.Warningf("failed to post data to center, url: %s, err: %v", url, err)
			continue
		}
		return nil
	}

	return fmt.Errorf("failed to post data to center, path= %s, addrs= %v", path, addrs)
}

func PostByUrlsWithResp[T any](ctx *ctx.Context, path string, v interface{}) (t T, err error) {
	addrs := ctx.CenterApi.Addrs
	if len(addrs) < 1 {
		err = fmt.Errorf("submission of the POST request from the center has failed, "+
			"path= %s, v= %v, ctx.CenterApi.Addrs= %v", path, v, addrs)
		return
	}

	// 随机选择起始位置
	startIdx := rand.Intn(len(addrs))

	// 从随机位置开始遍历所有地址
	for i := 0; i < len(addrs); i++ {
		idx := (startIdx + i) % len(addrs)
		url := fmt.Sprintf("%s%s", addrs[idx], path)

		t, err = PostByUrl[T](url, ctx.CenterApi, v)
		if err != nil {
			logger.Warningf("failed to post data to center, url: %s, err: %v", url, err)
			continue
		}
		return t, nil
	}

	return t, fmt.Errorf("failed to post data to center, path= %s, addrs= %v err: %v", path, addrs, err)
}

func PostByUrl[T any](url string, cfg conf.CenterApi, v interface{}) (t T, err error) {
	var bs []byte
	bs, err = json.Marshal(v)
	if err != nil {
		return
	}
	bf := bytes.NewBuffer(bs)
	if cfg.Timeout < 1 {
		cfg.Timeout = 5000
	}
	client := http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
	}

	if UseProxy(url) {
		client.Transport = ProxyTransporter
	}

	req, err := http.NewRequest("POST", url, bf)
	if err != nil {
		return t, fmt.Errorf("failed to create request %q: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json")

	if len(cfg.BasicAuthUser) > 0 {
		req.SetBasicAuth(cfg.BasicAuthUser, cfg.BasicAuthPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		return t, fmt.Errorf("failed to fetch from url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return t, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return t, fmt.Errorf("failed to read response body: %w", err)
	}

	var dataResp DataResponse[T]
	err = json.Unmarshal(body, &dataResp)
	if err != nil {
		return t, fmt.Errorf("failed to decode response: %w", err)
	}

	if dataResp.Err != "" {
		return t, fmt.Errorf("error from server: %s", dataResp.Err)
	}

	logger.Debugf("get data from %s, data: %+v", url, dataResp.Dat)
	return dataResp.Dat, nil

}

var ProxyTransporter = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
}

func UseProxy(url string) bool {
	// N9E_PROXY_URL=oapi.dingtalk.com,feishu.com
	patterns := os.Getenv("N9E_PROXY_URL")
	if patterns != "" {
		// 说明要让某些 URL 走代理
		for _, u := range strings.Split(patterns, ",") {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}

			if strings.Contains(url, u) {
				return true
			}
		}
	}
	return false
}

func PostJSON(url string, timeout time.Duration, v interface{}, retries ...int) (response []byte, code int, err error) {
	var bs []byte

	bs, err = json.Marshal(v)
	if err != nil {
		return
	}

	bf := bytes.NewBuffer(bs)

	client := http.Client{
		Timeout: timeout,
	}

	if UseProxy(url) {
		client.Transport = ProxyTransporter
	}

	req, err := http.NewRequest("POST", url, bf)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response

	if len(retries) > 0 {
		for i := 0; i < retries[0]; i++ {
			resp, err = client.Do(req)
			if err == nil {
				break
			}

			tryagain := ""
			if i+1 < retries[0] {
				tryagain = " try again"
			}

			logger.Warningf("failed to curl %s error: %s"+tryagain, url, err)

			if i+1 < retries[0] {
				time.Sleep(time.Millisecond * 200)
			}
		}
	} else {
		resp, err = client.Do(req)
	}

	if err != nil {
		return
	}

	code = resp.StatusCode

	if resp.Body != nil {
		defer resp.Body.Close()
		response, err = ioutil.ReadAll(resp.Body)
	}

	return
}
