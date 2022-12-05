package main

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/toriato/veto"
)

const decodeKey = "yL/M=zNa0bcPQdReSfTgUhViWjXkYIZmnpo+qArOBslCt2D3uE4Fv5G6wH178xJ9K"

var (
	patternKeys        = regexp.MustCompile(`_d\('([^']+)`)
	patternServiceCode = regexp.MustCompile(`service_code" value="([^"]+)`)
	patterns           = map[*regexp.Regexp]func(input string, matches []string) (*veto.Task, error){
		regexp.MustCompile(`https://gall\.dcinside\.com(/(mgallery|mini))?/board/view`): func(input string, _ []string) (*veto.Task, error) {
			u, err := url.Parse(input)
			if err != nil {
				return nil, err
			}

			q := u.Query()

			return &veto.Task{
				GalleryID: q.Get("id"),
				ArticleID: q.Get("no"),
			}, nil
		},
		regexp.MustCompile(`https://m\.dcinside\.com/(board/)?(mini/)?([^/]+)/(\d+)`): func(_ string, matches []string) (*veto.Task, error) {
			return &veto.Task{
				GalleryID: matches[3],
				ArticleID: matches[4],
			}, nil
		},
	}
)

func decode(body string) (string, error) {
	var keys, serviceCode string
	{
		matches := patternKeys.FindStringSubmatch(body)
		if len(matches) != 2 {
			return "", errors.New("보기 페이지에서 키 값을 찾을 수 없습니다")
		}

		keys = matches[1]

		matches = patternServiceCode.FindStringSubmatch(body)
		if len(matches) != 2 {
			return "", errors.New("보기 페이지에서 서비스 코드를 찾을 수 없습니다")
		}

		serviceCode = matches[1]
	}

	// common.js?v=210817:858
	k := [4]byte{}
	o := strings.Builder{}

	for c := 0; c < len(keys); {
		for i := 0; i < len(k); i++ {
			k[i] = byte(strings.Index(decodeKey, string(keys[c])))
			c += 1
		}

		o.WriteByte(k[0]<<2 | k[1]>>4)

		if k[2] != 64 {
			o.WriteByte((15&k[1])<<4 | k[2]>>2)
		}

		if k[3] != 64 {
			o.WriteByte((3&k[2])<<6 | k[3])
		}
	}

	// common.js?v=210817:862
	keys = o.String()
	fi, _ := strconv.Atoi(keys[0:1])

	if fi > 5 {
		fi -= 5
	} else {
		fi += 4
	}

	keys = string(rune(fi)) + keys

	var replacedServiceCode [10]rune
	for index, raw := range strings.Split(keys, ",") {
		floatChar, _ := strconv.ParseFloat(raw, 64)
		floatIndex := float64(index)
		char := 2 * (floatChar - floatIndex - 1) / (13 - floatIndex - 1)
		replacedServiceCode[index] = rune(char)
	}

	return serviceCode[:len(serviceCode)-9] + string(replacedServiceCode[:]), nil
}

func parseURL(url string) (*veto.Task, error) {
	for pattern, handler := range patterns {
		matches := pattern.FindStringSubmatch(url)

		if len(matches) > 0 {
			return handler(url, matches)
		}
	}

	return nil, nil
}

func checkAccounts() {
	wg := sync.WaitGroup{}

	logrus.Info("세션 검증을 시작합니다")

	for _, a := range opts.Accounts {
		sema.Acquire(context.Background(), 1)
		wg.Add(1)

		go func(a *veto.Account) {
			defer sema.Release(1)
			defer wg.Done()

			logger := logrus.WithField("username", a.Username[:3]+"*****")

			if ok, _ := a.LoggedIn(); ok {
				logger.Debug("기존 세션을 사용합니다")
				return
			}

			logger.Warn("세션이 만료됐습니다, 로그인을 시도합니다")

			for retries := 0; ; retries++ {
				a.Client.SetProxy("http://" + opts.Proxy.Provider.Next(nil))

				if err := a.Login(); err != nil {
					logger.WithField("retries", retries).Error(err)
					continue
				}

				break
			}

			logger.Info("새 세션을 가져왔습니다")
		}(a)
	}

	wg.Wait()

	logrus.Info("세션 검증이 완료됐습니다")

	if err := opts.Export(); err != nil {
		logrus.Error(err)
	}
}
