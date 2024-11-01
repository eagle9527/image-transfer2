package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"runtime"
	"tkestack.io/image-transfer/configs"
	tcr_image_transfer "tkestack.io/image-transfer/pkg/image-transfer"
	"tkestack.io/image-transfer/pkg/image-transfer/options"
	"tkestack.io/image-transfer/pkg/log"
)

//go:embed static/*
var staticFiles embed.FS

type Source map[string]configs.Security
type Target map[string]configs.Security

type ImageTransferRequest struct {
	Source Source            `json:"source"`
	Target Target            `json:"target"`
	Images map[string]string `json:"images"`
}

// 基本认证中间件
func basicAuth(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		const prefix = "Basic "
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix {
			c.Header("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			c.Header("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		pair := string(payload)
		if pair != username+":"+password {
			c.Header("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	r := gin.Default()

	// 添加 CORS 中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * 3600,
	}))

	// 添加基本认证中间件
	username := "admin"  // 替换为你的用户名
	password := "123456" // 替换为你的密码
	r.Use(basicAuth(username, password))

	// 提供静态文件服务
	r.StaticFS("/static", http.FS(staticFiles)) // 使用嵌入的文件系统

	r.POST("/image-transfer", func(c *gin.Context) {
		var req ImageTransferRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing request: %v", err)})
			return
		}

		opts := options.NewClientOptions()
		client, err := tcr_image_transfer.NewTransferClient(opts)
		if err != nil {
			log.Errorf("init Transfer Client error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize transfer client"})
			return
		}

		merged := make(map[string]configs.Security)
		for k, v := range req.Source {
			merged[k] = v
		}
		for k, v := range req.Target {
			merged[k] = v
		}

		client.Config.ImageList = req.Images
		client.Config.Security = merged
		client.Config.FlagConf.Config.RoutineNums = runtime.NumCPU()

		go func() {
			if err := client.Run(); err != nil {
				log.Error(fmt.Sprintf("Run failed:  %v\n", err.Error()))
			} else {
				log.Infof("Image transfer executed successfully")
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": "Image transfer executed successfully"})
	})

	// 设置路由以访问静态页面
	r.GET("/", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("static/index.html") // 读取嵌入的 HTML 文件
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load index.html"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	port := ":8080"
	fmt.Printf("Starting server on %s\n", port)
	if err := r.Run(port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
