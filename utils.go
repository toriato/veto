package veto

import (
	"fmt"
	"math/rand"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

const decodeKey = "yL/M=zNa0bcPQdReSfTgUhViWjXkYIZmnpo+qArOBslCt2D3uE4Fv5G6wH178xJ9K"

var (
	DefaultClient      = resty.New()
	patternKeys        = regexp.MustCompile(`_d\('([^']+)`)
	patternServiceCode = regexp.MustCompile(`service_code" value="([^"]+)`)
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

func cookie(jar *cookiejar.Jar, name string) (value string, ok bool) {
	// 쿠기 속에서 CSRF 토큰 가져오기
	for _, c := range jar.Cookies(&url.URL{Scheme: "https", Host: "gall.dcinside.com"}) {
		if c.Name == name {
			value = c.Value
			ok = true
			return
		}
	}

	return
}

func randHex() string {
	return fmt.Sprintf("%x", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64())
}
