package main

import "fmt"

// LogAlerter implements the Alerter interface for regular logging.
type LogAlerter struct {
}

// SendError sends the message as an error.
func (s *LogAlerter) SendError(msg string) error {
	// For now just send both as the same.
	return s.SendInfo(msg)
}

// SendInfo sends the message as info.
func (s *LogAlerter) SendInfo(msg string) error {
	fmt.Println(msg)
	return nil
}
