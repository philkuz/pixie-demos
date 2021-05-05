/*
 * Copyright 2018- The Pixie Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"github.com/slack-go/slack"
)

// SlackAlerter implements the Alerter interface for the SlackAPI.
type SlackAlerter struct {
	slackClient *slack.Client
	channel     string
}

// NewSlackAlerter returns a new slack alerter using the given slackToken.
func NewSlackAlerter(slackToken, slackChannel string) *SlackAlerter {
	slackClient := slack.New(slackToken)
	return &SlackAlerter{
		slackClient: slackClient,
		channel:     slackChannel,
	}
}

// SendError sends the message as an error.
func (s *SlackAlerter) SendError(msg string) error {
	// For now just send both as the same.
	return s.SendInfo(msg)
}

// SendInfo sends the message as info.
func (s *SlackAlerter) SendInfo(msg string) error {
	_, _, err := s.slackClient.PostMessage(s.channel, slack.MsgOptionText(msg, false), slack.MsgOptionAsUser(true))
	if err != nil {
		return err
	}
	return nil
}
