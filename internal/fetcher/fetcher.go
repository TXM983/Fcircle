package fetcher

import (
	"Fcircle/internal/model"
	"Fcircle/internal/utils"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

// LoadRemoteFriends 读取远程 JSON 配置
func LoadRemoteFriends(url string) ([]model.Friend, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("获取远程配置文件失败")
	}

	var friends []model.Friend
	if err := json.NewDecoder(resp.Body).Decode(&friends); err != nil {
		return nil, err
	}
	return friends, nil
}

// CrawlArticles 并发抓取所有友链的前N篇文章，返回FeedResult
func CrawlArticles(friends []model.Friend) map[string][]model.SimpleArticle {
	const maxConcurrency = 10
	const maxArticlesPerFriend = 10

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		allArticles []model.Article
	)

	sem := make(chan struct{}, maxConcurrency)

	for _, friend := range friends {
		wg.Add(1)
		sem <- struct{}{}

		go func(f model.Friend) {
			defer wg.Done()
			defer func() { <-sem }()

			articles, err := FetchFriendArticles(f, maxArticlesPerFriend)
			if err != nil {
				utils.Infof("抓取 [%s] 失败: %v\n", f.Name, err)
				return
			}

			utils.Infof("抓取 [%s] 成功，文章数: %d\n", f.Name, len(articles))

			mu.Lock()
			allArticles = append(allArticles, articles...)
			mu.Unlock()
		}(friend)
	}

	wg.Wait()

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}

	layout := "2006-01-02 15:04:05"

	sort.Slice(allArticles, func(i, j int) bool {
		t1, err1 := time.ParseInLocation(layout, allArticles[i].Published, loc)
		t2, err2 := time.ParseInLocation(layout, allArticles[j].Published, loc)

		if err1 != nil || err2 != nil {
			return false
		}
		return t1.After(t2)
	})

	// 构建最终 map[string][]SimpleArticle
	result := make(map[string][]model.SimpleArticle)

	for _, article := range allArticles {
		u, err := url.Parse(article.Url)
		if err != nil || u.Host == "" {
			continue
		}

		domain := u.Host
		simple := model.SimpleArticle{
			Title:  article.Title,
			Link:   article.Link,
			Source: domain,
			Date:   article.Published,
		}

		result[domain] = append(result[domain], simple)
	}

	return result
}
