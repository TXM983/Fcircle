package model

// Friend 代表一个友链的基本信息
type Friend struct {
	Name   string `json:"name"`   // 昵称
	URL    string `json:"url"`    // 个人站地址
	Avatar string `json:"avatar"` // 头像地址
	RSS    string `json:"RSS"`    // RSS 订阅源
}

// Article 代表从 RSS 中抓取的一篇文章
type Article struct {
	Title     string `json:"title"`     // 标题
	Link      string `json:"link"`      // 文章链接
	Published string `json:"published"` // 发布时间
	Author    string `json:"author"`    // 作者昵称
	Avatar    string `json:"avatar"`    // 作者头像
	Content   string `json:"content"`   // 内容
	Url       string `json:"url"`       // 个人站地址
}

// FeedResult 用于输出最终 JSON 文件结构
type FeedResult struct {
	Meta struct {
		FetchTime    string `json:"fetch_time"`    // 抓取时间
		FriendCount  int    `json:"friend_count"`  // 配置中友链数量
		SuccessCount int    `json:"success_count"` // 抓取成功的 RSS 数
		FailCount    int    `json:"fail_count"`    // 抓取失败的 RSS 数
		ArticleCount int    `json:"article_count"` // 总共抓取到的文章数
	} `json:"meta"`
	Items []Article `json:"items"` // 所有抓取到的文章列表
}

// SimpleArticle 定义新的输出 JSON文 件结构
type SimpleArticle struct {
	Title  string `json:"title"`
	Link   string `json:"link"`
	Source string `json:"source"`
	Date   string `json:"date"`
}
