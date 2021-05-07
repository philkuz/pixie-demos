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
	"context"
	"encoding/json"
	"os"
	"time"

	"px.dev/pxapi"
)

type slackBotConfig struct {
	PixieAPIKey    string `json:"PIXIE_API_KEY,omitempty"`
	PixieClusterID string `json:"PIXIE_CLUSTER_ID,omitempty"`
	SlackToken     string `json:"SLACK_BOT_TOKEN,omitempty"`
	SlackChannel   string
}

func loadConfigFromEnv() *slackBotConfig {
	// Slack channel for Slackbot to post in.
	// Slack App must be a member of this channel.
	slackChannel := "#pixie-alerts"
	// The slackbot requires the following configs, which are specified
	// using environment variables. For directions on how to find these
	// config values, see: https://docs.pixielabs.ai/tutorials/slackbot-alert
	pixieAPIKey, ok := os.LookupEnv("PIXIE_API_KEY")
	if !ok {
		panic("Please set PIXIE_API_KEY environment variable.")
	}

	pixieClusterID, ok := os.LookupEnv("PIXIE_CLUSTER_ID")
	if !ok {
		panic("Please set PIXIE_CLUSTER_ID environment variable.")
	}

	slackToken, ok := os.LookupEnv("SLACK_BOT_TOKEN")
	if !ok {
		panic("Please set SLACK_BOT_TOKEN environment variable.")
	}
	return &slackBotConfig{
		PixieAPIKey:    pixieAPIKey,
		PixieClusterID: pixieClusterID,
		SlackChannel:   slackChannel,
		SlackToken:     slackToken,
	}
}

func fromStdInJSON() (*slackBotConfig, error) {
	cfg := &slackBotConfig{}
	err := json.NewDecoder(os.Stdin).Decode(&cfg)
	if err != nil {
		return nil, err
	}
	cfg.SlackChannel = "#pixie-alerts"

	return cfg, nil
}

func main() {

	cfg, err := fromStdInJSON()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	pixieClient, err := pxapi.NewClient(ctx, pxapi.WithAPIKey(cfg.PixieAPIKey))
	if err != nil {
		panic(err)
	}
	vz, err := pixieClient.NewVizierClient(ctx, cfg.PixieClusterID)
	if err != nil {
		panic(err)
	}

	var alerter Alerter
	alerter = NewSlackAlerter(cfg.SlackToken, cfg.SlackChannel)
	if true {
		alerter.SendInfo("hi")
		return
	}
	// enable for testing.
	// alerter = &LogAlerter{}

	st, err := NewServiceTracker(alerter, vz)
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		err := st.Check(ctx)
		if err != nil {
			panic(err)
		}

		// wait for next tick
		// <-ticker.C
		break
	}
}
