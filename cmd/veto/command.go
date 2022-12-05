package main

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var ErrTaskUndefined = errors.New("작업이 없습니다")

var commands = map[string]func(args []string) error{
	"flags": func(args []string) error {
		for k, v := range opts.Flags {
			logrus.Infof("%s -> %v", k, v)
		}
		return nil
	},

	"flag": func(args []string) error {
		if len(args) < 2 {
			return errors.New("플래그 이름을 입력해주세요")
		}

		name := args[1]

		if _, ok := opts.Flags[name]; !ok {
			return errors.New("존재하지 않는 플래그입니다")
		}

		opts.Flags[name] = !opts.Flags[name]

		// 변경된 설정 저장하기
		if err := opts.Export(); err != nil {
			logrus.Error(err)
		}

		logrus.Infof("%s 플래그를 %v 값으로 변경했습니다", name, opts.Flags[name])

		return nil
	},

	"limits": func(args []string) (err error) {
		if len(args) > 1 {
			opts.Limits, err = strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return err
			}
		}

		if opts.Limits < 1 {
			logrus.Info("프록시 수만큼 작업을 수행합니다")
			opts.Limits = int64(opts.Proxy.Provider.Length())
		}

		// 변경된 설정 저장하기
		if err := opts.Export(); err != nil {
			logrus.Error(err)
		}

		logrus.Infof("limits = %d", opts.Limits)

		return nil
	},

	"concurrency": func(args []string) (err error) {
		if len(args) > 1 {
			opts.Concurrency, err = strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return err
			}

			// 변경된 설정 저장하기
			if err := opts.Export(); err != nil {
				logrus.Error(err)
			}
		}

		logrus.Infof("concurrency = %d", opts.Concurrency)
		return
	},
}

// func start() error {
// 	if task == nil {
// 		return ErrTaskUndefined
// 	}

// 	logrus.Info("작업을 시작합니다")

// 	task.Running = true

// 	leftMutex := sync.Mutex{}
// 	leftVotes := opts.Limits
// 	if !opts.Flags["vote"] {
// 		leftVotes = 0
// 	}

// 	leftComments := opts.Limits
// 	if !opts.Flags["comment"] {
// 		leftComments = 0
// 	}

// 	totalWaitGroup := sync.WaitGroup{}

// 	routineIdx := 0

// 	for task.Running {
// 		// 전체 작업이 끝났는지 확인하기
// 		leftMutex.Lock()
// 		totalLefts := leftVotes + leftComments
// 		leftMutex.Unlock()

// 		// 모든 작업이 스레드에서 실행 중이라면 대기하기
// 		if totalLefts < 1 {
// 			totalWaitGroup.Wait()

// 			// 완료 후에도 남은 작업이 없다면 반복문 종료하기
// 			if leftVotes+leftComments < 1 {
// 				logrus.Info("남은 작업이 없습니다, 작업을 멈춥니다")
// 				break
// 			}
// 		}

// 		if err := concurrency.Acquire(context.Background(), 1); err != nil {
// 			logrus.Error(err)
// 			continue
// 		}

// 		totalWaitGroup.Add(1)

// 		routineIdx++

// 		go func(routineIdx int) {
// 			defer concurrency.Release(1)
// 			defer totalWaitGroup.Done()

// 			var proxy string
// 			var cookieJar *cookiejar.Jar
// 			var client *resty.Client
// 			var serviceCode string

// 			logger := logrus.WithField("routine", routineIdx)

// 			for {
// 				proxy = opts.ProxyProvider.Next()
// 				logger = logger.WithField("proxy", proxy)

// 				cookieJar, _ = cookiejar.New(nil)
// 				client = resty.New().
// 					SetTimeout(2*time.Second).
// 					SetCookieJar(cookieJar).
// 					SetProxy("http://"+proxy).
// 					SetHeader("User-Agent", uarand.GetRandom())

// 					// 쿠키 불러오기
// 				res, err := task.FetchViewCookies(client)
// 				if err != nil {
// 					logger.Error(err)
// 					continue
// 				}

// 				// 서비스 코드 복호화하기
// 				serviceCode, err = decode(res.String())
// 				if err != nil {
// 					logger.Error(err)
// 					continue
// 				}

// 				break
// 			}

// 			// 현재 루틴에서 실행 중인 모든 작업에 대한 대기열
// 			routineWaitGroup := sync.WaitGroup{}
// 			defer routineWaitGroup.Wait()

// 			leftMutex.Lock()
// 			lockedLeftVotes := leftVotes
// 			leftMutex.Unlock()

// 			if lockedLeftVotes > 0 {
// 				routineWaitGroup.Add(1)

// 				leftMutex.Lock()
// 				leftVotes--
// 				leftMutex.Unlock()

// 				go func() {
// 					defer routineWaitGroup.Done()

// 					var result v.TaskVoteResult
// 					var account *v.Account

// 					// 남은 세션이 존재한다면 해당 세션 사용하기
// 					// TODO: 댓글 작성 플래그가 꺼져 있을 때만 남은 세션 사용, 이하 TODO 확인
// 					if !opts.Flags["downvote"] && !opts.Flags["comment"] && routineIdx < len(opts.Accounts) {
// 						account = opts.Accounts[routineIdx]

// 						logger = logger.WithField("username", account.Username[:3]+"*****")

// 						// TODO: 이후 댓글 작성할 때도 세션이 사용될 수 있음
// 						client.SetCookie(&http.Cookie{
// 							Domain: ".dcinside.com",
// 							Name:   "PHPSESSID",
// 							Value:  account.SessionID,
// 						})
// 					}

// 					// 세션(고닉)으로 추천한다면 동일한 계정으로 재시도할 필요 있음
// 					for {
// 						result = task.Vote(client, cookieJar, opts.Flags["downvote"])
// 						if result.Error != nil {
// 							logger.Error(result.Error)

// 							// 이미 투표한 상태라면 더 이상 시도하지 않기
// 							// 유동 상태에서 오류가 발생했다면 끝내기
// 							if errors.Is(result.Error, v.ErrTaskAlreadyVoted) || account == nil {
// 								leftMutex.Lock()
// 								leftVotes++
// 								leftMutex.Unlock()
// 								return
// 							}

// 							// 일반적으로 타임아웃이나 차단된 아이피로 실패하는 경우가 많으므로
// 							// 추천에 실패했다면 다음 프록시 사용하기
// 							proxy = proxies.Next()
// 							logger = logger.WithField("proxy", proxy)
// 							client.SetProxy("http://" + proxy)
// 							continue
// 						}

// 						break
// 					}

// 					logger.Infof("성공적으로 추천했습니다 (%d/%d)",
// 						result.Vote, result.VoteByRegisteredUser)

// 					if result.Recommended {
// 						logger.Info("성공적으로 주작했습니다!")

// 						if opts.Flags["autostop"] {
// 							task.Running = false
// 						}
// 					}
// 				}()
// 			}

// 			leftMutex.Lock()
// 			lockedLeftComments := leftComments
// 			leftMutex.Unlock()

// 			if lockedLeftComments > 0 {
// 				routineWaitGroup.Add(1)

// 				leftMutex.Lock()
// 				leftComments--
// 				leftMutex.Unlock()

// 				go func() {
// 					defer routineWaitGroup.Done()

// 					if err := task.CommentIcon(client, serviceCode); err != nil {
// 						leftMutex.Lock()
// 						leftComments++
// 						leftMutex.Unlock()

// 						logger.Error(err)
// 						return
// 					}

// 					logger.Infof("성공적으로 디시콘 댓글을 작성했습니다")
// 				}()
// 			}
// 		}(routineIdx)
// 	}

// 	logrus.Info("모든 스레드가 종료될 때까지 대기합니다")
// 	totalWaitGroup.Wait()
// 	logrus.Info("성공적으로 모든 작업을 멈췄습니다")

// 	task = nil
// 	return nil
// }
