package veto

import (
	"net/http/cookiejar"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Account struct {
	Client    *resty.Client  `yaml:"-"`
	CookieJar *cookiejar.Jar `yaml:"-"`
	Username  string         `yaml:"username"`
	Password  string         `yaml:"password"`
	SessionID string         `yaml:"session_id,omitempty"`
}

func (a *Account) Login() (err error) {
	referer := "https://sign.dcinside.com/login?s_url=https://gall.dcinside.com"

	if _, err = a.Client.R().Get(referer); err != nil {
		return errors.WithMessage(err, "로그인 쿠키 초기화를 위한 페이지 요청 중 오류가 발생했습니다")
	}

	var res *resty.Response
	res, err = a.Client.R().
		SetHeader("Referer", referer).
		SetFormData(map[string]string{
			"user_id": a.Username,
			"pw":      a.Password,
			"s_url":   referer,
		}).
		Post("https://sign.dcinside.com/login/member_check")
	if err != nil {
		return
	}

	logrus.Debug(res.String())

	var ok bool
	if a.SessionID, ok = cookie(a.CookieJar, "PHPSESSID"); !ok {
		return errors.New("세션 아이디가 없습니다")
	}

	if ok, err = a.LoggedIn(); err != nil {
		return
	}

	if !ok {
		err = errors.New("로그인에 실패했습니다")
	}

	return
}

func (a Account) LoggedIn() (bool, error) {
	res, err := a.Client.R().Get("https://gallog.dcinside.com")
	if err != nil {
		return false, errors.WithMessage(err, "로그인 상태 확인을 위한 갤로그 페이지 요청 중 오류가 발생했습니다")
	}

	// 정상적으로 로그인되지 않은 세션이라면 갤로그 접속 시 로그인 페이지로 리다이렉션됨
	if strings.Contains(res.String(), "sign.dcinside.com") {
		return false, nil
	}

	return true, nil
}
