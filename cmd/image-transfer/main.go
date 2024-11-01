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
	"fmt"
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

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	r := gin.Default()

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
		if err := client.Run(); err != nil {
			log.Error(fmt.Sprintf("Run failed:  %v\n", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Image transfer failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Image transfer executed successfully"})
	})

	port := ":8080"
	fmt.Printf("Starting server on %s\n", port)
	if err := r.Run(port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}