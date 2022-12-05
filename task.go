package veto

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"time"

	fakeua "github.com/eddycjy/fake-useragent"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

type GalleryType string

const (
	GalleryTypeMajor GalleryType = "G"
	GalleryTypeMinor GalleryType = "M"
	GalleryTypeMini  GalleryType = "MI"
)

type Task struct {
	sync.Mutex

	Running     bool
	Runned      int
	Endpoint    string
	GalleryType GalleryType
	GalleryID   string
	ArticleID   string

	context context.Context
}

type TaskVoteResult struct {
	Success              bool
	Recommended          bool
	Vote                 int
	VoteByRegisteredUser int
	Error                error
}

func (task *Task) referer() string {
	return fmt.Sprintf("%s/board/view/?id=%s&no=%s",
		task.Endpoint, task.GalleryID, task.ArticleID)
}

func (task *Task) Key() string {
	return fmt.Sprintf("%s/%s", task.GalleryID, task.ArticleID)
}

func (task *Task) Fetch() error {
	res, err := DefaultClient.R().Get("https://gall.dcinside.com/board/lists/?id=" + task.GalleryID)
	if err != nil {
		return errors.WithMessage(err, "갤러리 목록 페이지 요청 중 오류가 발생했습니다")
	}

	if res.StatusCode() == 404 {
		res, err = DefaultClient.R().Get("https://gall.dcinside.com/mini/board/lists/?id=" + task.GalleryID)
		if err != nil {
			return errors.WithMessage(err, "미니 갤러리 목록 페이지 요청 중 오류가 발생했습니다")
		}

		if res.StatusCode() == 404 {
			return errors.New("존재하지 않는 갤러리입니다")
		}

		task.GalleryType = GalleryTypeMini
	} else {
		if strings.Contains(res.String(), "/mgallery/") {
			task.GalleryType = GalleryTypeMinor
		} else {
			task.GalleryType = GalleryTypeMajor
		}
	}

	task.Endpoint = "https://gall.dcinside.com"

	switch task.GalleryType {
	case GalleryTypeMinor:
		task.Endpoint += "/mgallery"
	case GalleryTypeMini:
		task.Endpoint += "/mini"
	}

	return nil
}

func (task *Task) FetchViewCookies(client *resty.Client) (*resty.Response, error) {
	return client.R().Get(task.referer())
}

var (
	ErrTaskAlreadyVoted   = errors.New("이미 추천했습니다")
	ErrTaskInvalidCaptcha = errors.New("캡챠가 잘못됐습니다")
	ErrTaskBanned         = errors.New("접근이 차단된 아이피입니다")
	ErrTaskNotFound       = errors.New("게시글이 존재하지 않습니다")
)

func (task *Task) Vote(client *resty.Client, jar *cookiejar.Jar, downvote bool) (r TaskVoteResult) {
	token, ok := cookie(jar, "ci_c")
	if !ok {
		r.Error = errors.New("토큰을 찾을 수 없습니다")
		return
	}

	payload := map[string]string{
		"ci_t":           token,
		"_GALLTYPE_":     string(task.GalleryType),
		"link_id":        task.GalleryID,
		"id":             task.GalleryID,
		"no":             task.ArticleID,
		"mode":           "U",
		"code_recommend": "",
	}

	cookieNameFormat := "%s%s_Firstcheck"

	if downvote {
		payload["mode"] = "D"
		cookieNameFormat = "%s%s_Firstcheck_down"
	}

	client.SetCookie(&http.Cookie{
		Domain: ".dcinside.com",
		Name:   fmt.Sprintf(cookieNameFormat, task.GalleryID, task.ArticleID),
		Value:  "Y",
	})

	res, err := client.R().
		SetContext(task.context).
		SetHeaders(map[string]string{
			"Referer":          task.referer(),
			"Content-Type":     "application/x-www-form-urlencoded",
			"X-Requested-With": "XMLHttpRequest",
		}).
		SetFormData(payload).
		Post("https://gall.dcinside.com/board/recommend/vote")
	if err != nil {
		r.Error = errors.WithMessage(err, "투표 요청 중 오류가 발생했습니다")
		return
	}

	result := strings.Split(res.String(), "||")

	if len(result) > 1 {
		if result[0] == "true" {
			r.Success = true
			r.Vote, _ = strconv.Atoi(result[1])

			if !downvote {
				r.VoteByRegisteredUser, _ = strconv.Atoi(result[2])

				if result[3] == "true" {
					r.Recommended = true
				}
			}
		} else {
			switch result[1] {
			case "추천은 1일 1회만 가능합니다.":
				r.Error = ErrTaskAlreadyVoted
			case "자동입력 방지코드가 일치하지 않습니다.":
				r.Error = ErrTaskInvalidCaptcha
			case "잘못된 접근입니다.":
				r.Error = ErrTaskBanned
			default:
				r.Error = errors.New(result[1])
			}

		}

		return
	}

	r.Error = errors.New("투표 요청 페이지에서 예측하지 못한 데이터를 반환했습니다")
	return
}

func (task *Task) Comment(client *resty.Client, serviceCode string) error {
	_, err := client.R().
		SetContext(task.context).
		SetHeaders(map[string]string{
			"Referer":          task.referer(),
			"Content-Type":     "application/x-www-form-urlencoded",
			"X-Requested-With": "XMLHttpRequest",
		}).
		SetFormData(map[string]string{
			"_GALLTYPE_":   string(task.GalleryType),
			"id":           task.GalleryID,
			"no":           task.ArticleID,
			"c_gall_id":    task.GalleryID,
			"c_gall_no":    task.ArticleID,
			"check_6":      "6" + randHex(),
			"check_7":      "7" + randHex(),
			"check_8":      "8" + randHex(),
			"check_9":      "9" + randHex(),
			"name":         "ㅇㅇ",
			"password":     "dltkdgk98",
			"memo":         strconv.FormatInt(time.Now().UnixMilli(), 10),
			"service_code": serviceCode,
		}).
		Post("https://gall.dcinside.com/board/forms/comment_submit")
	if err != nil {
		return errors.WithMessage(err, "댓글 작성 요청 중 오류가 발생했습니다")
	}

	// TODO: 결과 값 처리하기
	return nil
}

func (task *Task) CommentIcon(client *resty.Client, serviceCode string) error {
	res, err := client.R().
		SetContext(task.context).
		SetHeaders(map[string]string{
			"Referer":          task.referer(),
			"Content-Type":     "application/x-www-form-urlencoded",
			"X-Requested-With": "XMLHttpRequest",
		}).
		SetFormData(map[string]string{
			"_GALLTYPE_":   string(task.GalleryType),
			"id":           task.GalleryID,
			"no":           task.ArticleID,
			"c_gall_id":    task.GalleryID,
			"c_gall_no":    task.ArticleID,
			"name":         "ㅇㅇ",
			"password":     "dltkdgk98",
			"package_idx":  "92234",
			"detail_idx":   "2521062",
			"input_type":   "comment",
			"check_6":      "6" + randHex(),
			"check_7":      "7" + randHex(),
			"check_8":      "8" + randHex(),
			"service_code": serviceCode,
		}).
		Post("https://gall.dcinside.com/dccon/insert_icon")
	if err != nil {
		return errors.WithMessage(err, "댓글 작성 요청 중 오류가 발생했습니다")
	}

	if strings.Contains(res.String(), "fail") {
		return errors.New("디시콘 댓글 작성에 실패했습니다")
	}

	// TODO: 결과 값 처리하기
	return nil
}

func (task *Task) Start(opts *Options) error {
	wg := sync.WaitGroup{}
	sema := semaphore.NewWeighted(opts.Concurrency)
	logger := logrus.WithField("key", task.Key())

	votes := opts.Limits
	if !opts.Flags["vote"] {
		votes = 0
	}

	comments := opts.Limits
	if !opts.Flags["comment"] {
		comments = 0
	}

	task.Running = true

	var cancel context.CancelFunc
	task.context, cancel = context.WithCancel(context.Background())
	defer cancel()

	for {
		// 전체 작업이 끝났는지 확인하기
		task.Lock()
		lefts := votes + comments
		task.Unlock()

		if !task.Running {
			logger.Info("작업이 강제로 종료됐습니다, 진행 중인 작업이 모두 종료될 때까지 대기합니다")
			cancel()
			wg.Wait()
			break
		}

		// 모든 작업이 스레드에서 실행 중이라면 대기하기
		if lefts < 1 {
			logger.Info("남은 작업이 없습니다, 진행 중인 작업이 모두 종료될 때까지 대기합니다")
			wg.Wait()

			// 완료 후에도 남은 작업이 없다면 반복문 종료하기
			if votes+comments < 1 {
				break
			}
		}

		if err := sema.Acquire(context.Background(), 1); err != nil {
			logger.Error(err)
			continue
		}

		task.Runned++

		wg.Add(1)

		go func(runned int) {
			defer sema.Release(1)
			defer wg.Done()

			var proxy string
			var cookieJar *cookiejar.Jar
			var client *resty.Client
			var serviceCode string

			logger := logger.WithField("runned", runned)

			for {
				proxy = opts.Proxy.Provider.Next(opts.Proxy.Ignores)

				cookieJar, _ = cookiejar.New(nil)
				client = resty.New().
					SetTimeout(2*time.Second).
					SetCookieJar(cookieJar).
					SetProxy("http://"+proxy).
					SetHeader("User-Agent", fakeua.Computer())

				// 쿠키 불러오기
				res, err := task.FetchViewCookies(client)
				if err != nil {
					// 중요하지 않은 로그는 디버깅 상태에서만 출력하기
					if errors.Is(err, context.Canceled) {
						logger.Debug(err)
					} else if t, ok := err.(net.Error); ok && t.Timeout() {
						logger.Debug(t)
					} else {
						logger.Error(err)
					}

					continue
				}

				// 게시글이 존재하지 않는다면 작업 종료하기
				if res.StatusCode() == 404 || strings.Contains(res.String(), "/derror/deleted") {
					task.Running = false
					logger.Error("게시글이 존재하지 않습니다, 작업을 종료합니다")
					return
				}

				// 서비스 코드 복호화하기
				serviceCode, err = decode(res.String())
				if err != nil {
					logger.Error(err)
					continue
				}

				break
			}

			// 현재 루틴에서 실행 중인 모든 작업에 대한 대기열
			routineWaitGroup := sync.WaitGroup{}
			defer routineWaitGroup.Wait()

			task.Lock()
			lockedVotes := votes
			task.Unlock()

			if lockedVotes > 0 {
				routineWaitGroup.Add(1)

				task.Lock()
				votes--
				task.Unlock()

				go func() {
					defer routineWaitGroup.Done()

					var result TaskVoteResult
					var account *Account

					// 남은 세션이 존재한다면 해당 세션 사용하기
					// TODO: 댓글 작성 플래그가 꺼져 있을 때만 남은 세션 사용, 이하 TODO 확인
					if !opts.Flags["downvote"] && !opts.Flags["comment"] && runned < len(opts.Accounts) {
						account = opts.Accounts[runned]

						logger = logger.WithField("username", account.Username[:3]+"*****")

						// TODO: 이후 댓글 작성할 때도 세션이 사용될 수 있음
						client.SetCookie(&http.Cookie{
							Domain: ".dcinside.com",
							Name:   "PHPSESSID",
							Value:  account.SessionID,
						})
					}

					// 세션(고닉)으로 추천한다면 동일한 계정으로 재시도 필요함
					for {
						result = task.Vote(client, cookieJar, opts.Flags["downvote"])
						if result.Error != nil {
							// 중요하지 않은 로그는 디버깅 상태에서만 출력하기
							if errors.Is(result.Error, context.Canceled) {
								logger.Debug(result.Error)
							} else if t, ok := result.Error.(net.Error); ok && t.Timeout() {
								logger.Debug(t)
							} else {
								logger.Error(result.Error)
							}

							switch {

							// 캡챠 인증이 필요하다면 멈추기
							case errors.Is(result.Error, ErrTaskInvalidCaptcha):
								return

							// 차단된 아이피라면 제외 목록에 추가하기
							case errors.Is(result.Error, ErrTaskBanned):
								opts.Lock()
								opts.Proxy.Ignores = append(opts.Proxy.Ignores, proxy)
								opts.Unlock()

							// 이미 투표한 상태라면 더 이상 시도하지 않기
							// 유동 상태에서 오류가 발생했다면 끝내기
							case
								errors.Is(result.Error, ErrTaskAlreadyVoted),
								!task.Running,
								account == nil:

								task.Lock()
								votes++
								task.Unlock()
								return
							}

							// 일반적으로 타임아웃이나 차단된 아이피로 실패하는 경우가 많으므로
							// 추천에 실패했다면 다음 프록시 사용하기
							proxy = opts.Proxy.Provider.Next(opts.Proxy.Ignores)
							client.SetProxy("http://" + proxy)
							continue
						}

						break
					}

					logger.Infof("성공적으로 추천했습니다 (%d/%d)",
						result.Vote, result.VoteByRegisteredUser)

					if result.Recommended {
						logger.Info("성공적으로 주작했습니다!")

						task.Lock()
						task.Running = false
						task.Unlock()
					}
				}()
			}

			task.Lock()
			lockedComments := comments
			task.Unlock()

			if lockedComments > 0 {
				routineWaitGroup.Add(1)

				task.Lock()
				comments--
				task.Unlock()

				go func() {
					defer routineWaitGroup.Done()

					if err := task.CommentIcon(client, serviceCode); err != nil {
						task.Lock()
						comments++
						task.Unlock()

						logger.Error(err)
						return
					}

					logger.Infof("성공적으로 디시콘 댓글을 작성했습니다")
				}()
			}
		}(task.Runned)
	}

	logger.Info("작업이 끝났습니다")
	return nil
}
