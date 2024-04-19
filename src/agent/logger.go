package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

var logger = log.WithFields(log.Fields{"agent": "lokiLogger"})

// LokiLogger 구조체를 정의합니다. Loki 서버의 URL과 필요한 설정을 포함합니다.
type LokiLogger struct {
	functionName string
	requestId string
	lokiAddress string
	httpClient  *http.Client
}

// NewLokiLogger 함수는 새로운 LokiLogger 인스턴스를 생성하고 초기화합니다.
func NewLokiLogger() (*LokiLogger, error) {
	fName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	lokiAddress, ok := os.LookupEnv("LOKI_PUSH_URL") // 환경변수에서 Loki 서버 주소를 가져옵니다.
	if !ok {
		return nil, errors.New("LOKI_PUSH_URL is not set")
	}

	return &LokiLogger{
		functionName: fName,
		lokiAddress: lokiAddress,
		httpClient:  &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (l *LokiLogger) SetRequestId(requestId string) {
	l.requestId = requestId
}

// PushLog 메소드는 로그 메시지를 Loki 서버로 전송합니다.
func (l *LokiLogger) PushLog(logEntries [][]string) error {
	if len(logEntries) == 0 {
		return nil // 로그 엔트리가 비어있는 경우 바로 반환
	}

	streams := make(map[string][][2]string)
	for _, entry := range logEntries {
		level := entry[2]
		streams[level] = append(streams[level], [2]string{entry[0], entry[1]})
	}

	var logDataStreams []map[string]interface{}
	for level, values := range streams {
		stream := map[string]interface{}{
			"stream": map[string]string{
				"job": "lambda",
				"function_name": l.functionName,
				"request_id": l.requestId,
				"level": level,
			},
			"values": values,
		}
		logDataStreams = append(logDataStreams, stream)
	}

	logData := map[string]interface{}{
		"streams": logDataStreams,
	}

	data, err := json.Marshal(logData)
	if err != nil {
		return err
	}

	// Loki로 로그 데이터를 전송합니다.
	req, err := http.NewRequest("POST", l.lokiAddress+"/loki/api/v1/push", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	authToken, ok := os.LookupEnv("LOKI_AUTH_TOKEN")
	if ok {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to push log to Loki, status code: %d", resp.StatusCode)
	}

	return nil
}

// Shutdown 메서드는 LokiLogger를 정상적으로 종료합니다.
func (l *LokiLogger) Shutdown() error {
	// 이 예제에서는 추가적인 정리 작업이 필요하지 않습니다.
	// 필요한 경우, 여기에 정리 코드를 추가합니다.
	return nil
}