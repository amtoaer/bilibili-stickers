package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	errOpenFile      = "读取表情包列表失败"
	errParseFile     = "解析表情包列表失败"
	errGetInfo       = "获取表情包信息失败"
	errRetryDownload = "第%d次下载%s失败，正在重试\n"
	errDownloadImg   = "下载%s失败\n"
	errFindFormat    = "识别%s表情格式失败：%s\n"
	successDownload  = "下载%s成功\n"
)

// Downloader 图标包下载器
type Downloader struct {
	wg sync.WaitGroup
}

// Download 下载单个表情包
func (d *Downloader) Download(stickerInfo map[string]interface{}) {
	name, ok := stickerInfo["text"].(string) // 读取表情包名称
	if !ok {
		panic(errGetInfo)
	}
	dirName := path.Join("stickers", name)
	os.MkdirAll(dirName, os.FileMode(0755)) // 为每个表情包创建单独文件夹
	stickerItems, ok := stickerInfo["emote"].([]interface{})
	if !ok {
		panic(errGetInfo)
	}
	for idx := range stickerItems { // 遍历单个表情
		d.wg.Add(1)
		go func(index int) { // 串行下载表情包，并行下载表情包内单个表情
			item, ok := stickerItems[index].(map[string]interface{})
			if !ok {
				panic(errGetInfo)
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
				dotPos := strings.LastIndexByte(link, '.') // 在链接中找到.的位置
				if dotPos == -1 {
					log.Printf(errFindFormat, iconName, link)
					continue
				}
				fileName := path.Join(dirName,
					iconName+link[dotPos:])
				if _, err := os.Stat(fileName); os.IsNotExist(err) {
					for i := 1; i <= 3; i++ { // 重试三次
						if resp, err := http.Get(link); err == nil {
							defer resp.Body.Close()
							if img, err := ioutil.ReadAll(resp.Body); err == nil {
								ioutil.WriteFile(fileName, img, fs.FileMode(0644))
								log.Printf(successDownload, fileName)
								break
							}
						} else {
							if i == 3 {
								log.Printf(errDownloadImg, fileName)
								break
							}
							log.Printf(errRetryDownload, i, fileName)
							time.Sleep(3 * time.Second)
						}
					}
				}
			}
			d.wg.Done()
		}(idx)
	}
	d.wg.Wait()
}

func main() {
	if len(os.Args) <= 1 {
		return
	}
	content, err := ioutil.ReadFile("./stickers.json")
	if err != nil {
		panic(errOpenFile)
	}
	var tmp map[string]interface{}
	if err = json.Unmarshal(content, &tmp); err != nil {
		panic(errParseFile)
	}
	stickers := (tmp["data"].(map[string]interface{}))["all_packages"].([]interface{})
	switch os.Args[1] {
	case "list":
		for idx := range stickers {
			stickerInfo := stickers[idx].(map[string]interface{})
			fmt.Printf("%d. %s\n", idx, stickerInfo["text"].(string))
		}
	default:
		downloader := Downloader{}
		if os.Args[1] == "all" {
			for idx := range stickers {
				stickerInfo := stickers[idx].(map[string]interface{})
				downloader.Download(stickerInfo)
			}
		} else {
			if idx, err := strconv.Atoi(os.Args[1]); err == nil {
				stickerInfo := stickers[idx].(map[string]interface{})
				downloader.Download(stickerInfo)
			}
		}
	}
}
