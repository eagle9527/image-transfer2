/*
 * Tencent is pleased to support the open source community by making TKEStack
 * available.
 *
 * Copyright (C) 2012-2020 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package main

import (
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
		// 获取请求中的Authorization头
		auth := c.Request.Header.Get("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", "Basic realm=\"Authorization Required\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 解码Authorization头
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

		// 将解码后的payload拆分成用户名和密码
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
		AllowOrigins:  []string{"*"},                                // 允许所有域名
		AllowMethods:  []string{"GET", "POST", "OPTIONS"},           // 允许的方法
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept"}, // 允许的请求头
		ExposeHeaders: []string{"Content-Length"},                   // 允许暴露的头
		MaxAge:        12 * 3600,                                    // 预检请求的最大有效期（秒）
	}))

	// 添加基本认证中间件
	username := "admin"  // 替换为你的用户名
	password := "123456" // 替换为你的密码
	r.Use(basicAuth(username, password))

	// 提供静态文件服务
	r.Static("/static", "./static") // 假设你的 HTML 文件存放在 ./static 目录下

	r.POST("/image-transfer", func(c *gin.Context) {
		var req ImageTransferRequest

		// 解析 JSON 请求体
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

		// 合并两个 map
		merged := make(map[string]configs.Security)
		// 将 source 的键值对添加到 merged
		for k, v := range req.Source {
			merged[k] = v
		}

		// 将 target 的键值对添加到 merged
		for k, v := range req.Target {
			merged[k] = v
		}

		client.Config.ImageList = req.Images
		client.Config.Security = merged
		client.Config.FlagConf.Config.RoutineNums = runtime.NumCPU()

		// 异步执行
		go func() {
			if err := client.Run(); err != nil {
				log.Error(fmt.Sprintf("Run failed:  %v\n", err.Error()))
				// 处理错误（如发送通知等）
			} else {
				log.Infof("Image transfer executed successfully")
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": "Image transfer executed successfully"})
	})

	// 设置路由以访问静态页面
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html") // 假设你的 HTML 文件是 index.html
	})

	port := ":8080"
	fmt.Printf("Starting server on %s\n", port)
	if err := r.Run(port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
