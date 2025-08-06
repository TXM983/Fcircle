package main

import (
	"Fcircle/internal/config"
	"Fcircle/internal/fetcher"
	"Fcircle/internal/middleware"
	"Fcircle/internal/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	fetchMutex sync.Mutex
	isFetching bool
	appConfig  *config.AppConfig
)

func main() {

	_ = os.Setenv("TZ", "Asia/Shanghai")

	// 启动后台清理任务
	middleware.InitRateLimiterCleanup(30 * time.Minute)

	appConfig = config.LoadConfig()

	err := utils.InitLog(appConfig.Log.File)
	if err != nil {
		log.Fatalf("日志初始化失败：%v", err)
	}

	fmt.Println("程序启动，开始首次抓取...")
	go fetchAndSave()

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}

	c := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(loc),
	)
	_, err = c.AddFunc(appConfig.Task.CronExpr, fetchAndSave)
	if err != nil {
		fmt.Println("定时任务添加失败：", err)
		return
	}
	c.Start()

	fmt.Printf("定时任务添加成功，Cron 表达式为： %v\n", appConfig.Task.CronExpr)

	r := gin.New()

	r.Use(
		gin.Recovery(),
	)

	r.GET("/fetch", middleware.RateLimit(2, 24*time.Hour), ginFetchHandler)
	r.GET("/feed", middleware.RateLimit(5, 2*time.Minute), ginFeedResultHandler)

	addr := fmt.Sprintf(":%d", appConfig.Server.Port)
	fmt.Printf("HTTP服务启动，监听端口 %d\n", appConfig.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatalf("HTTP服务启动失败: %v\n", err)
	}
}

func fetchAndSave() {
	fetchMutex.Lock()
	if isFetching {
		fmt.Println("抓取任务正在执行中，请稍后...")
		fetchMutex.Unlock()
		return
	}
	isFetching = true
	fetchMutex.Unlock()

	defer func() {
		fetchMutex.Lock()
		isFetching = false
		fetchMutex.Unlock()
	}()

	friends, err := fetcher.LoadRemoteFriends(appConfig.RSS.ConfigURL)
	if err != nil {
		fmt.Printf("加载友链配置失败: %v\n", err)
		os.Exit(1)
	}

	result := fetcher.CrawlArticles(friends)

	err = utils.WriteToFile(appConfig.RSS.OutputFile, result)
	if err != nil {
		fmt.Printf("写入结果文件失败: %v\n", err)
		os.Exit(1)
	}
	total := 0
	for _, articles := range result {
		total += len(articles)
	}

	fmt.Printf("抓取完成，共 %d 篇文章，结果写入 %s\n", total, appConfig.RSS.OutputFile)
}

func ginFetchHandler(c *gin.Context) {

	secretKey := appConfig.Server.SecretKey
	queryKey := c.Query("key")
	if queryKey != secretKey {
		c.JSON(http.StatusForbidden, gin.H{"error": "无效的访问密钥"})
		return
	}

	fetchMutex.Lock()
	if isFetching {
		fetchMutex.Unlock()
		c.JSON(http.StatusTooEarly, gin.H{
			"message": "抓取任务正在执行中，请稍后再试",
		})
		return
	}
	isFetching = true
	fetchMutex.Unlock()

	go func() {
		defer func() {
			fetchMutex.Lock()
			isFetching = false
			fetchMutex.Unlock()
		}()

		fmt.Println("HTTP接口触发抓取任务开始...")

		friends, err := fetcher.LoadRemoteFriends(appConfig.RSS.ConfigURL)
		if err != nil {
			fmt.Println("加载友链配置失败:", err)
			return
		}

		result := fetcher.CrawlArticles(friends)

		err = utils.WriteToFile(appConfig.RSS.OutputFile, result)
		if err != nil {
			fmt.Println("写入结果文件失败:", err)
			return
		}
		total := 0
		for _, articles := range result {
			total += len(articles)
		}

		fmt.Printf("HTTP触发抓取完成，共 %d 篇文章\n", total)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "抓取任务已启动",
	})
}

func ginFeedResultHandler(c *gin.Context) {
	filePath := appConfig.RSS.OutputFile
	data, err := os.ReadFile(filePath)
	if err != nil {
		utils.Errorf("读取feed结果文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "读取数据失败，请稍后重试",
		})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}
