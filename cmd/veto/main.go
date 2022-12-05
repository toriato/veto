package main

import (
	"bufio"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toriato/veto"
	"golang.org/x/sync/semaphore"
)

var (
	opts  = &veto.Options{}
	tasks = map[string]*veto.Task{}
	sema  *semaphore.Weighted
)

func main() {
	if err := opts.Import(); err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("프록시 %d개를 무시합니다", len(opts.Proxy.Ignores))

	sema = semaphore.NewWeighted(opts.Concurrency)

	go func() {
		p := opts.Proxy.Provider

		for {
			p.Lock()

			if err := p.LoggedIn(); err != nil {
				logrus.Error(err)

				if err := p.Login(); err != nil {
					logrus.Error(err)
				} else if err := p.Fetch(); err != nil {
					logrus.Error(err)
				} else {
					logrus.Infof("프록시 제공자로부터 프록시 %d개를 가져왔습니다", p.Length())
				}
			}

			p.Unlock()

			time.Sleep(1 * time.Minute)
		}
	}()

	go func() {
		for {
			checkAccounts()
			time.Sleep(5 * time.Minute)
		}
	}()

	go func() {
		reader := bufio.NewReader(os.Stdin)

		for {
			input, _ := reader.ReadString('\n')
			input = input[0 : len(input)-1]
			args := strings.Split(input, " ")

			// 주소 파싱하기
			t, err := parseURL(input)
			if err != nil {
				logrus.Error(err)
			} else if t != nil {
				// 작업할 게시글 정보 불러오기
				if err := t.Fetch(); err != nil {
					logrus.Error(err)
					continue
				}

				// 작업 목록에 작업 추가하기
				tasks[t.Key()] = t
				go func() {
					t.Start(opts)
					delete(tasks, t.Key())
				}()

				continue
			}

			// 명령어 존재하면 명령어 실행하기
			if handler, ok := commands[args[0]]; ok {
				if err := handler(args); err != nil {
					logrus.Error(err)
				}

				continue
			}

			logrus.Errorf("'%s' 는 존재하는 명령어가 아닙니다", args[0])
		}
	}()

	go func() {
		http.HandleFunc("/request", handleRequest)
		http.ListenAndServe(":8123", nil)
	}()

	for {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
		<-c

		// 현재 진행 중인 작업이 있다면 모두 중단하기
		if len(tasks) > 0 {
			for _, task := range tasks {
				task.Lock()
				task.Running = false
				task.Unlock()
			}
			continue
		}

		// 없다면 프로그램 종료
		opts.Export()
		os.Exit(0)
	}
}
