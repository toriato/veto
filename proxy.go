package veto

import (
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

const (
	proxyProviderEndpoint = "http://agent.cleanip.net/client/proxy"
)

type ProxyProvider struct {
	sync.Mutex `yaml:"-"`

	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	MaxRetries int    `yaml:"max_retries"`

	proxies       []string `yaml:"-"`
	proxiesLength int      `yaml:"-"`
	cursor        int      `yaml:"-"`

	remoteIP string `yaml:"-"` // 로그인 후 서버에서 반환 받은 외부 아이피
	authKey  string `yaml:"-"` // 로그인 인증 키
}

// 로그인을 시도합니다
func (p *ProxyProvider) Login() error {
	params := url.Values{}
	params.Set("id", p.Username)
	params.Set("pwd", p.Password)
	params.Set("force", "true")

	resp, err := http.Get(proxyProviderEndpoint + "/api_login.php?" + params.Encode())
	if err != nil {
		return errors.WithMessage(err, "프록시 제공자 로그인 요청 중 오류가 발생했습니다")
	}

	var lines []string
	{
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.WithMessage(err, "프록시 제공자 로그인 데이터를 읽는 중 오류가 발생했습니다")
		}

		lines = strings.Split(string(b), "\n")
	}

	if len(lines) < 3 || lines[0] != "TRUE" {
		return errors.New("프록시 제공자 로그인에 실패했습니다")
	}

	p.remoteIP = lines[1]
	p.authKey = lines[2]
	return nil
}

// 로그인 된 상태인지 확인합니다
func (p *ProxyProvider) LoggedIn() error {
	params := url.Values{}
	params.Set("id", p.Username)
	params.Set("remote_ip", p.remoteIP)
	params.Set("auth_flag", p.authKey)

	resp, err := http.Get(proxyProviderEndpoint + "/api_login_chk.php?" + params.Encode())
	if err != nil {
		return errors.WithMessage(err, "프록시 제공자 인증 검증 요청 중 오류가 발생했습니다")
	}

	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.WithMessage(err, "프록시 제공자 인증 검증 데이터를 읽는 중 오류가 발생했습니다")
	}

	if !strings.Contains(string(b), "TRUE") {
		return errors.New("프록시 제공자 인증에 실패했습니다")
	}

	return nil
}

// 프록시 아이피 목록을 불러옵니다
func (p *ProxyProvider) Fetch() error {
	params := url.Values{}
	params.Set("id", p.Username)
	params.Set("remote_ip", p.remoteIP)
	params.Set("auth_flag", p.authKey)
	params.Set("type", "0")

	resp, err := http.Get(proxyProviderEndpoint + "/api_iplist.php?" + params.Encode())
	if err != nil {
		return errors.WithMessage(err, "프록시 제공자 목록 요청 중 오류가 발생했습니다")
	}

	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.WithMessage(err, "프록시 제공자 목록 데이터를 읽는 중 오류가 발생했습니다")
	}

	lines := strings.Split(string(b), "\n")

	// 중복 프록시 제거하기
	for _, proxy := range lines[2 : len(lines)-1] {
		for _, duped := range p.proxies {
			if proxy == duped {
				proxy = ""
				break
			}
		}

		if proxy != "" {
			p.proxies = append(p.proxies, proxy)
		}
	}

	p.proxiesLength = len(p.proxies)
	p.cursor = 0

	// 불러온 프록시 목록 섞기
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(p.proxies), func(i, j int) {
		p.proxies[i], p.proxies[j] = p.proxies[j], p.proxies[i]
	})

	return nil
}

func (p *ProxyProvider) Length() int {
	return p.proxiesLength
}

// 프록시 목록에서 이전과 다른 프록시를 하나를 가져옵니다
func (p *ProxyProvider) Next(ignores []string) string {
	// 고루틴에서 불러올 때 중복되지 않도록 락하기
	p.Lock()
	defer p.Unlock()

	var proxy string

	for {
		p.cursor++
		if p.cursor >= p.proxiesLength {
			p.cursor = 0
		}

		proxy = p.proxies[p.cursor]

		if ignores == nil {
			break
		}

		for _, p := range ignores {
			if proxy == p {
				proxy = ""
			}
		}

		if proxy != "" {
			break
		}
	}

	return proxy
}

// 새로운 프록시가 적용된 웹 클라이언트를 만들어 반환합니다
func (p *ProxyProvider) Client() *resty.Client {
	// var client *resty.Client

	// for retries := 0; retries < p.MaxRetries; retries++ {
	// 	c := resty.New()
	// 	c.SetTimeout(5 * time.Second)
	// 	c.SetProxy("http://" + p.Next())

	// 	_, err := c.R().Head("https://api.ipify.org/")
	// 	if err != nil {
	// 		p.Logger.Warn(errors.WithMessage(err, "프록시 테스트 요청 중 오류가 발생했습니다"))
	// 		continue
	// 	}

	// 	client = c
	// 	break
	// }

	// if client == nil {
	// 	return nil, ResultError.Assign(errors.New("정상적인 프록시를 찾을 수 없습니다"))
	// }

	client := resty.New()
	client.SetTimeout(5 * time.Second)
	client.SetProxy("http://" + p.Next(nil))

	return client
}
