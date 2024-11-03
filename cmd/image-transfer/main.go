package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// 全局通道，用于通知日志清空
var logCleared = make(chan struct{}, 1)

// WebSocket 处理程序
func logWSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to upgrade connection: %v", err))
		return
	}
	defer conn.Close()

	// 打开日志文件
	logFile, err := os.Open("./logs/app.log")
	if err != nil {
		log.Error(fmt.Sprintf("Failed to open log file: %v", err))
		return
	}
	defer logFile.Close()

	// 初始时移动到日志文件末尾
	_, err = logFile.Seek(0, io.SeekEnd)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to seek to end of log file: %v", err))
		return
	}

	buf := make([]byte, 1024)
	for {
		select {
		case <-logCleared:
			// 如果日志被清空，重置文件指针
			logFile.Close()
			logFile, err = os.Open("./logs/app.log") // 重新打开日志文件
			if err != nil {
				log.Error(fmt.Sprintf("Failed to open log file after clearing: %v", err))
				return
			}
			_, err = logFile.Seek(0, io.SeekEnd) // 移动到文件末尾
			if err != nil {
				log.Error(fmt.Sprintf("Failed to seek to end of log file after clearing: %v", err))
				return
			}
		default:
			n, err := logFile.Read(buf)
			if err != nil {
				if err == io.EOF {
					time.Sleep(1 * time.Second) // 等待新的日志条目
					continue
				}
				log.Error(fmt.Sprintf("Error reading log file: %v", err))
				break
			}

			if n > 0 {
				if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
					log.Error(fmt.Sprintf("Failed to write message: %v", err))
					break
				}
			}
		}
	}
}

// 清空日志文件的具体实现
func clearLogFile(logFilePath string) error {
	// 打开文件，使用 os.O_TRUNC 来清空内容
	file, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Infof("Failed to open log file: %v", err)
		return err
	}
	defer file.Close()

	log.Infof("Log file %s cleared successfully.", logFilePath)
	return nil
}

func clearLogHandler(c *gin.Context) {
	logFilePath := "./logs/app.log"

	// 清空日志文件
	err := clearLogFile(logFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear log file"})
		return
	}

	// 通知 WebSocket 处理程序日志已被清空
	select {
	case logCleared <- struct{}{}: // 发送日志清空信号
	default: // 如果通道满，什么都不做
	}

	c.JSON(http.StatusOK, gin.H{"message": "Log file cleared successfully"})
}

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
	username := "admin"        // 替换为你的用户名
	password := "RKO6G6VBH0R5" // 替换为你的密码
	r.Use(basicAuth(username, password))

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

	// 提供 CSS 文件
	r.GET("/static/css/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		data, err := staticFiles.ReadFile("static/css" + filepath) // 读取嵌入的 CSS 文件
		if err != nil {
			log.Errorf("Error reading CSS file: %v", err) // 打印具体错误
			c.JSON(http.StatusNotFound, gin.H{"error": "CSS file not found"})
			return
		}
		c.Data(http.StatusOK, "text/css; charset=utf-8", data)
	})

	// 提供 JS 文件
	r.GET("/static/js/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		data, err := staticFiles.ReadFile("static/js" + filepath) // 读取嵌入的 JS 文件
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "JS file not found"})
			return
		}
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", data)
	})

	// WebSocket 路由
	r.GET("/ws/logs", logWSHandler)

	r.POST("/clear-log", clearLogHandler)

	port := ":8080"
	fmt.Printf("Starting server on %s\n", port)
	if err := r.Run(port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
