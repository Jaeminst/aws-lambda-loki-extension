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
	"strings"
	"syscall"

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

			logEntries := []LogEntry{}
			err = json.Unmarshal([]byte(logsStr), &logEntries)
			if err != nil {
					fmt.Println("Error unmarshalling JSON:", err)
					return
			}

			var log string = ""
			for _, entry := range logEntries {
				switch record := entry.Record.(type) {
				case string:
						// Record가 문자열인 경우
						fmt.Printf("Time: %s, Type: %s, Record: %s\n", entry.Time, entry.Type, record)
						if entry.Type == "function" {
							log += record + "\n"
						}
				case map[string]interface{}:
						// Record가 JSON 객체인 경우
						recordBytes, err := json.Marshal(record)
						if err != nil {
								fmt.Println("Error marshalling record:", err)
								continue
						}
						fmt.Printf("Time: %s, Type: %s, Record: %s\n", entry.Time, entry.Type, string(recordBytes))
						if entry.Type == "function" {
							log += string(recordBytes) + "\n"
						}
				default:
						fmt.Println("Unknown Record type")
				}
			}

			err = lokiLogger.PushLog(log)
			if err != nil {
				logger.Error(printPrefix, err)
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
