// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT-0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"loki-logs/agent"
	"loki-logs/extension"
	"loki-logs/logsapi"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang-collections/go-datastructures/queue"
	log "github.com/sirupsen/logrus"
)

// INITIAL_QUEUE_SIZE is the initial size set for the synchronous logQueue
const INITIAL_QUEUE_SIZE = 5

func main() {
	extensionName := path.Base(os.Args[0])
	printPrefix := fmt.Sprintf("[%s]", extensionName)
	logger := log.WithFields(log.Fields{"agent": extensionName})

	extensionClient := extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sigs
		cancel()
		logger.Info(printPrefix, "Received", s)
		logger.Info(printPrefix, "Exiting")
	}()

	// Register extension as soon as possible
	_, err := extensionClient.Register(ctx, extensionName)
	if err != nil {
		panic(err)
	}

	// Create Loki Logger
	lokiLogger, err := agent.NewLokiLogger()
	if err != nil {
		logger.Fatal(err)
	}

	// A synchronous queue that is used to put logs from the goroutine (producer)
	// and process the logs from main goroutine (consumer)
	logQueue := queue.New(INITIAL_QUEUE_SIZE)
	// Helper function to empty the log queue
	var logsStr string = ""
	flushLogQueue := func(force bool) {
		for !(logQueue.Empty() && (force || strings.Contains(logsStr, string(logsapi.RuntimeDone)))) {
			logs, err := logQueue.Get(1)
			if err != nil {
				logger.Error(printPrefix, err)
				return
			}
			logsStr = fmt.Sprintf("%v", logs[0])

			type LogEntry struct {
				Time   string      `json:"time"`
				Type   string      `json:"type"`
				Record interface{} `json:"record"`
			}

			var logEntries []LogEntry
			err = json.Unmarshal([]byte(logsStr), &logEntries)
			if err != nil {
					logger.Error(printPrefix, "Error unmarshalling JSON:", err)
					return
			}

			var preparedLogs [][]string
			for _, entry := range logEntries {
				if entry.Type == "function" {
					recordStr, ok := entry.Record.(string)
					if !ok {
						logger.Error(printPrefix, "Record is not a string")
						continue
					}

					var level string = "LOGS"
					normalizedRecord := strings.ToLower(recordStr)
					if strings.Contains(normalizedRecord, "level") {
						var jsonData map[string]interface{}
						err := json.Unmarshal([]byte(recordStr), &jsonData)
						if err == nil {
							// Unmarshal 성공: input은 JSON 형태입니다.
							switch lvl := jsonData["level"].(type) {
							case float64:
								level = strconv.FormatFloat(lvl, 'f', 0, 64)
							case string:
								level = lvl
							}
							message, messageOk := jsonData["message"].(string)
							if messageOk {
								// level과 message를 사용합니다.
								recordStr = fmt.Sprintf("%s\t%s", level, message)
							}
						} else {
							// Unmarshal 실패: input은 일반 문자열입니다.
							parts := strings.Split(recordStr, "\t")
							if len(parts) > 2 {
								level = parts[2]
								recordStr = strings.Join(parts[2:], "\t")
							}
						}
					} else {
            if strings.Contains(normalizedRecord, "fatal") {
							level = "FATAL"
            } else if strings.Contains(normalizedRecord, "error") {
							level = "ERROR"
            } else if strings.Contains(normalizedRecord, "warn") {
							level = "WARN"
            } else if strings.Contains(normalizedRecord, "info") {
							level = "INFO"
            } else if strings.Contains(normalizedRecord, "debug") {
							level = "DEBUG"
            }
					}

					t, err := time.Parse(time.RFC3339Nano, entry.Time)
					if err != nil {
						fmt.Println("Error parsing time:", err)
						return
					}
					unixNano := strconv.FormatInt(t.UnixNano(), 10)
					preparedLogs = append(preparedLogs, []string{unixNano, recordStr, level})
				}
			}

			err = lokiLogger.PushLog(preparedLogs)
			if err != nil {
				logger.Error(printPrefix, "Error pushing logs to Loki:", err)
				return
			}
		}
	}

	// Create Logs API agent with LokiLogger
	logsApiAgent, err := agent.NewHttpAgent(lokiLogger, logQueue)
	if err != nil {
		logger.Fatal(err)
	}
	// Subscribe to logs API
	// Logs start being delivered only after the subscription happens.
	agentID := extensionClient.ExtensionID
	err = logsApiAgent.Init(agentID)
	if err != nil {
		logger.Fatal(err)
	}

	// Will block until invoke or shutdown event is received or cancelled via the context.
	for {
		select {
		case <-ctx.Done():
			return
		default:
			logger.Info(printPrefix, " Waiting for event...")
			// This is a blocking call
			res, err := extensionClient.NextEvent(ctx)
			if err != nil {
				logger.Info(printPrefix, "Error:", err)
				logger.Info(printPrefix, "Exiting")
				return
			}
			lokiLogger.SetRequestId(res.RequestID)
			logger.Info(printPrefix, " Invoke Function")
			// Flush log queue in here after waking up
			flushLogQueue(false)
			// Exit if we receive a SHUTDOWN event
			if res.EventType == extension.Shutdown {
				logger.Info(printPrefix, "Received SHUTDOWN event")
				flushLogQueue(true)
				logsApiAgent.Shutdown()
				logger.Info(printPrefix, "Exiting")
				return
			}
		}
	}
}
