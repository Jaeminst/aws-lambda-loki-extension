package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var logger = log.WithFields(log.Fields{"agent": "lokiLogger"})

// LokiLogger 구조체를 정의합니다. Loki 서버의 URL과 필요한 설정을 포함합니다.
type LokiLogger struct {
	lokiAddress string
	httpClient  *http.Client
}

// NewLokiLogger 함수는 새로운 LokiLogger 인스턴스를 생성하고 초기화합니다.
func NewLokiLogger() *LokiLogger {
	lokiAddress := os.Getenv("LOKI_ADDRESS") // 환경변수에서 Loki 서버 주소를 가져옵니다.
	if lokiAddress == "" {
		lokiAddress = "http://localhost:3100" // 기본 Loki 서버 주소
	}

	return &LokiLogger{
		lokiAddress: lokiAddress,
		httpClient:  &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// PushLog 메소드는 로그 메시지를 Loki 서버로 전송합니다.
func (l *LokiLogger) PushLog(logEntry string) error {
	// Loki에 보낼 로그 데이터를 생성합니다.
	logData := map[string]interface{}{
		"streams": []map[string]interface{}{
			{
				"stream": map[string]string{
					"source": "lambda", // 로그 소스를 식별하는 데 사용할 추가 필드를 추가할 수 있습니다.
					// 여기에 더 많은 라벨을 추가할 수 있습니다.
				},
				"values": [][]string{
					{strconv.FormatInt(time.Now().UnixNano(), 10), logEntry},
				},
			},
		},
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