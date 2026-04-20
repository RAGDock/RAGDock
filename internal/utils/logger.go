package utils

import (
	"fmt"
	"time"
)

// Log formats and prints a log message with a 24-hour timestamp: HH:MM:SS:ms
func Log(tag string, message string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05:000")
	fullMsg := fmt.Sprintf(message, args...)
	fmt.Printf("%s | [%s] | %s\n", timestamp, tag, fullMsg)
}

// LogRaw prints a message without extra tagging if needed, but still with the timestamp
func LogRaw(message string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05:000")
	fullMsg := fmt.Sprintf(message, args...)
	fmt.Printf("%s | %s\n", timestamp, fullMsg)
}
