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

package log

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger          *zap.Logger
	once            sync.Once
	logFilePath     = "./logs/app.log" // 指定日志文件路径
	logWithColor    = false            // 是否启用彩色日志
	logIgnoreCaller = false            // 是否忽略调用者信息
	logSamplingFreq = time.Millisecond // 采样频率
)

// InitLogger initializes logger the way we want for tke.
func InitLogger() {
	once.Do(func() {
		logger = newLogger()
	})
}

// FlushLogger calls the underlying Core's Sync method, flushing any buffered
// log entries. Applications should take care to call Sync before exiting.
func FlushLogger() {
	if logger != nil {
		// #nosec
		// nolint: errcheck
		logger.Sync()
	}
}

// ZapLogger returns zap logger instance.
func ZapLogger() *zap.Logger {
	return getLogger()
}

// Reset to recreate the logger by changed flag params
func Reset() {
	once.Do(func() {
		logger = newLogger()
	})
}

// Check return if logging a message at the specified level is enabled.
func Check(level int32) bool {
	var lvl zapcore.Level
	if level < 5 {
		lvl = zapcore.InfoLevel
	} else {
		lvl = zapcore.DebugLevel
	}
	checkEntry := getLogger().Check(lvl, "")
	return checkEntry != nil
}

// Debug method output debug level log.
func Debug(msg string, fields ...zapcore.Field) {
	getLogger().Debug(msg, fields...)
}

// Info method output info level log.
func Info(msg string, fields ...zapcore.Field) {
	getLogger().Info(msg, fields...)
}

// Warn method output warning level log.
func Warn(msg string, fields ...zapcore.Field) {
	getLogger().Warn(msg, fields...)
}

// Error method output error level log.
func Error(msg string, fields ...zapcore.Field) {
	getLogger().Error(msg, fields...)
}

// Panic method output panic level log and shutdown application.
func Panic(msg string, fields ...zapcore.Field) {
	getLogger().Panic(msg, fields...)
}

// Fatal method output fatal level log.
func Fatal(msg string, fields ...zapcore.Field) {
	getLogger().Fatal(msg, fields...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	Debug(fmt.Sprintf(template, args...))
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	Info(fmt.Sprintf(template, args...))
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	Warn(fmt.Sprintf(template, args...))
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	Error(fmt.Sprintf(template, args...))
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	Panic(fmt.Sprintf(template, args...))
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	Fatal(fmt.Sprintf(template, args...))
}

func getLogger() *zap.Logger {
	once.Do(func() {
		logger = newLogger()
	})
	return logger
}

func newLogger() *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建一个自定义编码器
	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10240, // megabytes
		MaxBackups: 0,
		MaxAge:     0, // days
	})

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig), // 使用控制台编码器
		writer,                                   // 输出到文件
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.InfoLevel
		}),
	)

	l := zap.New(core, zap.AddStacktrace(zapcore.PanicLevel), zap.AddCaller())

	return l
}
