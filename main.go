package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	errGetCookie     = "读取 cookie 失败，请设置命令行参数 -cookie"
	errGetEmoji      = "获取表情包列表失败：%v"
	errParseEmoji    = "解析表情包列表失败：%v"
	errGetInfo       = "获取表情包信息失败"
	errRetryDownload = "第%d次下载%s失败，正在重试：%v"
	errDownloadImg   = "下载%s失败：%s"
	errParseLink     = "解析%s表情下载地址失败：%s"
	successDownload  = "下载%s成功"
	fileAlreadyExist = "文件%s已存在，跳过下载"
)

var (
	client *http.Client = nil
	// from template: https://github.com/charmbracelet/bubbletea/blob/master/examples/send-msg/main.go
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	dotStyle      = helpStyle.Copy().UnsetMargins()
	durationStyle = dotStyle.Copy()
	appStyle      = lipgloss.NewStyle().Margin(1, 2, 0, 2)
)

func randomEmoji() string {
	emojis := []string{
		"🍔",
		"🍕",
		"🌭",
		"🍣",
		"🍦",
		"🍩",
		"🍪",
		"🍎",
		"🍌",
		"🍇",
	}
	return emojis[rand.Intn(len(emojis))]
}

type resultMsg struct {
	emoji    string
	msg      string
	quit     bool
	err      error
	duration time.Duration
}

func (r resultMsg) String() string {
	if r.duration == 0 {
		return dotStyle.Render(strings.Repeat(".", 30))
	}
	var msg string

	if r.msg != "" {
		msg = r.msg
	} else {
		msg = r.err.Error()
	}
	return fmt.Sprintf("%s %s %s", r.emoji, msg,
		durationStyle.Render(r.duration.String()))
}

type model struct {
	spinner spinner.Model
	results []resultMsg
	quit    bool
	abort   bool
	error   bool
}

func newModel() model {
	const numLastResults = 8
	s := spinner.New()
	s.Style = spinnerStyle
	return model{
		spinner: s,
		results: make([]resultMsg, numLastResults),
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.abort = true
			return m, tea.Quit
		}
		return m, nil
	case resultMsg:
		if msg.quit {
			m.quit = true
			return m, tea.Quit
		} else if msg.err != nil {
			m.results = append(m.results[1:], msg)
			m.error = true
			return m, tea.Quit
		}
		m.results = append(m.results[1:], msg)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) View() string {
	var s string

	if m.quit {
		s += "下载已完成！"
	} else if m.abort {
		s += "下载被中断！"
	} else if m.error {
		s += m.spinner.View() + "执行出现错误！"
	} else {
		s += m.spinner.View() + " 下载表情包中..."
	}

	s += "\n\n"

	for _, res := range m.results {
		s += res.String() + "\n"
	}

	if m.quit || m.error {
		s += "\n"
	}

	return appStyle.Render(s)
}

func initClient(cookie string) {
	url, _ := url.Parse("https://api.bilibili.com")
	cookieJar, _ := cookiejar.New(nil)
	cookieJar.SetCookies(url, []*http.Cookie{
		{
			Name:  "SESSDATA",
			Value: cookie,
		},
	})
	client = &http.Client{
		Timeout: 10 * time.Second,
		Jar:     cookieJar,
	}
}

// Downloader 图标包下载器
type Downloader struct {
	wg sync.WaitGroup
}

// Download 下载单个表情包
func (d *Downloader) Download(p *tea.Program, startTime time.Time, stickerInfo map[string]interface{}) {
	name, ok := stickerInfo["text"].(string) // 读取表情包名称
	if !ok {
		p.Send(resultMsg{emoji: randomEmoji(), msg: errGetInfo, duration: time.Since(startTime)})
		return
	}
	dirName := path.Join("stickers", name)
	os.MkdirAll(dirName, os.FileMode(0755)) // 为每个表情包创建单独文件夹
	stickerItems, ok := stickerInfo["emote"].([]interface{})
	if !ok {
		p.Send(resultMsg{emoji: randomEmoji(), msg: errGetInfo, duration: time.Since(startTime)})
		return
	}
	for idx := range stickerItems { // 遍历单个表情
		d.wg.Add(1)
		go func(index int) { // 串行下载表情包，并行下载表情包内单个表情
			item, ok := stickerItems[index].(map[string]interface{})
			if !ok {
				p.Send(resultMsg{emoji: randomEmoji(), msg: errGetInfo, duration: time.Since(startTime)})
			}
			var links []string
			staticLink, linkOk := item["url"].(string) // 静态表情
			gifLink, gifOk := item["gif_url"].(string) //gif 动态表情
			if linkOk {
				links = append(links, staticLink)
			}
			if gifOk {
				links = append(links, gifLink)
			}
			for _, link := range links {
				iconName := item["text"].(string)
				dotPos := strings.LastIndexByte(link, '.')
				isLink := dotPos != -1 && strings.HasPrefix(link, "http") // 有 http 前缀并且有 . 符号的认为是链接
				if !isLink {
					p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(errParseLink, iconName, link), duration: time.Since(startTime)})
					continue
				}
				fileName := path.Join(dirName, iconName+link[dotPos:])
				if _, err := os.Stat(fileName); os.IsNotExist(err) {
					for i := 1; i <= 3; i++ { // 重试三次
						if resp, err := client.Get(link); err == nil {
							defer resp.Body.Close()
							if img, err := io.ReadAll(resp.Body); err == nil {
								os.WriteFile(fileName, img, fs.FileMode(0644))
								p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(successDownload, fileName), duration: time.Since(startTime)})
								break
							}
						} else {
							if i == 3 {
								p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(errDownloadImg, fileName, err), duration: time.Since(startTime)})
								break
							}
							p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(errRetryDownload, i, fileName, err), duration: time.Since(startTime)})
							time.Sleep(3 * time.Second)
						}
					}
				} else {
					p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(fileAlreadyExist, fileName), duration: time.Since(startTime)})
				}
			}
			d.wg.Done()
		}(idx)
	}
	d.wg.Wait()
}

func main() {
	p := tea.NewProgram(newModel())

	sessData := flag.String("sessdata", "", "你的 SESSDATA")
	flag.Parse()

	startTime := time.Now()

	go func() {
		if *sessData == "" && os.Getenv("SESSDATA") == "" {
			p.Send(resultMsg{emoji: randomEmoji(), err: fmt.Errorf(errGetCookie), duration: time.Since(startTime)})
			return
		}
		if *sessData == "" {
			*sessData = os.Getenv("SESSDATA")
		}
		initClient(*sessData)
		resp, err := client.Get("https://api.bilibili.com/x/emote/setting/panel?business=reply")
		if err != nil {
			p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(errGetEmoji, err), duration: time.Since(startTime)})
			return
		}
		content, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var tmp map[string]interface{}
		if err = json.Unmarshal(content, &tmp); err != nil {
			p.Send(resultMsg{emoji: randomEmoji(), msg: fmt.Sprintf(errParseEmoji, err), duration: time.Since(startTime)})
		}
		stickers := (tmp["data"].(map[string]interface{}))["all_packages"].([]interface{})
		downloader := Downloader{}
		for idx := range stickers {
			stickerInfo := stickers[idx].(map[string]interface{})
			downloader.Download(p, startTime, stickerInfo)
		}
		p.Send(resultMsg{emoji: randomEmoji(), quit: true})
	}()
	if _, err := p.Run(); err != nil {
		os.Exit(1)
	}
}
