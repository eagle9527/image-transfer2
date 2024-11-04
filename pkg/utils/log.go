package utils

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
	"tkestack.io/image-transfer/pkg/log"
)

// 清空日志文件的具体实现
func ClearLogFile(logFilePath string) error {
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// 全局通道，用于通知日志清空
var logCleared = make(chan struct{}, 1)

// WebSocket 处理程序
func LogWSHandler(c *gin.Context) {
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

func ClearLogHandler(c *gin.Context) {
	logFilePath := "./logs/app.log"

	// 清空日志文件
	err := ClearLogFile(logFilePath)
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

func BasicAuth(username, password string) gin.HandlerFunc {
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

// 获取一个未使用的随机端口
func GetRandomPort() (string, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return strconv.Itoa(port), nil
}
