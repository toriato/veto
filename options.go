package veto

import (
	"net/http"
	"net/http/cookiejar"
	"os"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Options struct {
	sync.Mutex

	Flags       map[string]bool `yaml:"flags"`
	Limits      int64
	Concurrency int64
	Proxy       struct {
		Provider *ProxyProvider `yaml:"provider"`
		Ignores  []string
	} `yaml:"proxy"`
	Accounts []*Account `yaml:"accounts"`
}

func (opts *Options) Import() error {
	opts.Lock()
	defer opts.Unlock()

	b, err := os.ReadFile("veto.yml")
	if err != nil {
		return errors.WithMessage(err, "설정 파일을 읽는 중 오류가 발생했습니다")
	}

	if err := yaml.Unmarshal(b, opts); err != nil {
		return errors.WithMessage(err, "설정 파일을 파싱하는 중 오류가 발생했습니다")
	}

	for _, account := range opts.Accounts {
		account.CookieJar, _ = cookiejar.New(nil)
		account.Client = resty.New().SetCookieJar(account.CookieJar)

		if account.SessionID != "" {
			account.Client.SetCookie(&http.Cookie{
				Domain: ".dcinside.com",
				Name:   "PHPSESSID",
				Value:  account.SessionID,
			})
		}
	}

	return nil
}

func (opts *Options) Export() error {
	opts.Lock()
	defer opts.Unlock()

	b, err := yaml.Marshal(opts)
	if err != nil {
		return errors.WithMessage(err, "설정을 파일화 하는 중 오류가 발생했습니다")
	}

	if err := os.WriteFile("veto.yml", b, 0o644); err != nil {
		return errors.WithMessage(err, "설정 파일을 쓰는 중 오류가 발생했습니다")
	}

	return nil
}
