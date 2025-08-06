package fetcher

import (
	"Fcircle/internal/model"
	"Fcircle/internal/utils"
	"fmt"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
	"net/http"
	"strings"
	"time"
)

// FetchFriendArticles 请求并解析指定 friend 的 RSS，返回最新 maxCount 篇文章
func FetchFriendArticles(friend model.Friend, maxCount int) ([]model.Article, error) {

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
		},
	}

	const maxRetry = 3

	var (
		resp *http.Response
		err  error
	)

	start := time.Now()

	userAgents := []string{
		"FcircleBot/1.0 (+https://github.com/TXM983/Fcircle)",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 12; Pixel 6 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	}

	for attempt := 0; attempt <= maxRetry; attempt++ {
		currentUA := userAgents[attempt%len(userAgents)]

		req, e := http.NewRequest("GET", friend.RSS, nil)
		if e != nil {
			err = e
			break
		}

		req.Header.Set("User-Agent", currentUA)

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break // 成功
		}

		if resp != nil {
			resp.Body.Close()
		}

		// 等待再重试
		time.Sleep(2 * time.Second)
	}
	duration := time.Since(start)

	utils.Infof("抓取 [%s] RSS 用时: %v", friend.Name, duration)

	if err != nil {
		return nil, fmt.Errorf("RSS 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS 请求失败，状态码: %d", resp.StatusCode)
	}

	parser := gofeed.NewParser()
	feed, err := parser.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("RSS 解析失败: %v", err)
	}

	articles := make([]model.Article, 0, maxCount)
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}

	for i, item := range feed.Items {
		if i >= maxCount {
			break
		}

		pubTime := time.Now().In(loc)
		if item.PublishedParsed != nil {
			pubTime = item.PublishedParsed.In(loc)
		} else if item.UpdatedParsed != nil {
			pubTime = item.UpdatedParsed.In(loc)
		}

		formattedTime := pubTime.Format("2006-01-02 15:04:05")

		author := friend.Name
		if item.Author != nil && item.Author.Name != "" {
			author = item.Author.Name
		}

		content := ""
		if item.Content != "" {
			content = item.Content
		} else if item.Description != "" {
			content = item.Description
		}

		cleanContent := ExtractCleanHTML(content)      // 清理html标签
		cleanContent = safeTruncate(cleanContent, 250) // 字符截取

		article := model.Article{
			Title:     item.Title,
			Link:      item.Link,
			Published: formattedTime,
			Author:    author,
			Avatar:    friend.Avatar,
			Content:   cleanContent,
			Url:       friend.URL,
		}
		articles = append(articles, article)
	}

	return articles, nil
}

func safeTruncate(s string, maxChars int) string {
	cleaned := strings.ToValidUTF8(s, "")
	runes := []rune(cleaned)
	if len(runes) <= maxChars {
		return cleaned
	}

	truncated := string(runes[:maxChars])
	return fixBrokenHTML(truncated)
}

func fixBrokenHTML(s string) string {
	if lastOpen := strings.LastIndex(s, "<"); lastOpen != -1 {
		if lastClose := strings.LastIndex(s, ">"); lastClose < lastOpen {
			s = s[:lastOpen]
		}
	}

	aOpenCount := strings.Count(s, "<a")
	aCloseCount := strings.Count(s, "</a>")
	if aOpenCount > aCloseCount {
		s += "</a>"
	}

	return s
}

func ExtractCleanHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}

	var builder strings.Builder

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			builder.WriteString(n.Data)

		case html.ElementNode:
			// 保留 <a> 和 <br>
			switch n.Data {
			case "a":
				builder.WriteString(`<a`)
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						builder.WriteString(` href="` + html.EscapeString(attr.Val) + `"`)
					}
				}
				builder.WriteString(`>`)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					traverse(c)
				}
				builder.WriteString(`</a>`)

			case "br":
				builder.WriteString(`<br>`)

			default:
				// 忽略其他标签，仅递归内容
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					traverse(c)
				}
			}

		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				traverse(c)
			}
		}
	}

	traverse(doc)
	return strings.TrimSpace(builder.String())
}
