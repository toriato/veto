package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
)

const token = "29d22712686a798cfa153786ff21e4ad1216a3cbe7fed4326e0ec269d107ae69"

type Payload struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	payload := Payload{}
	status := http.StatusInternalServerError

	defer func() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(payload)
	}()

	if r.Header.Get("Authorization") != token {
		status = http.StatusUnauthorized
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer r.Body.Close()

	input := string(b)

	// 작업이 없다면 주소 파싱하기
	for pattern, handler := range patterns {
		matches := pattern.FindStringSubmatch(input)

		if len(matches) > 0 {
			t, err := handler(input, matches)
			if err != nil {
				logrus.Error(err)
			} else if err := t.Fetch(); err != nil {
				logrus.Error(err)
			} else {
				tasks[t.Key()] = t
				go func() {
					t.Start(opts)
					delete(tasks, t.Key())
				}()

				status = http.StatusOK
				payload.OK = true
				return
			}

			break
		}
	}

	status = http.StatusBadRequest
	payload.Message = "Unsupported URL"
}
