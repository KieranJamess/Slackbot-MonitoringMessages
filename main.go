package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/tkanos/gonfig"
)

var (
	infoLogger    = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	warningLogger = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime)
	errorLogger   = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	debugLogger   = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
)

type Config struct {
	SlackAccessToken  string `json:"slack_access_token"`
	ApprovalEmoji     string `json:"approval_emoji"`
	ReviewingEmoji    string `json:"reviewing_emoji"`
	ReviewingInterval int    `json:"reviewing_interval"`
}

type SlackMessage struct {
	Channel           string   `json:"channel"`
	Mentions          []string `json:"mentions"`
	Message           string   `json:"message"`
	ReminderIntervals *int     `json:"reminderIntervals"`
}

func makeMentionList(users []string) string {
	var list strings.Builder
	for _, user := range users {
		user = fmt.Sprintf("<@%s>", user)
		list.WriteString(user)
	}
	return list.String()
}

func sendSlackMessage(channel, message, messageTs string, api *slack.Client) string {
	_, messageTs, err := api.PostMessage(channel, slack.MsgOptionText(message, false), slack.MsgOptionTS(messageTs))
	if err != nil {
		errorLogger.Println("Error sending message:", err)
	}
	return messageTs
}

func handleMessage(write http.ResponseWriter, request *http.Request, defaultInterval int, api *slack.Client) {
	decoder := json.NewDecoder(request.Body)
	var message SlackMessage
	err := decoder.Decode(&message)
	if err != nil {
		errorLogger.Println("Error decoding JSON:", err)
		return
	}

	// Set default interval if message is missing this field
	if message.ReminderIntervals == nil {
		debugLogger.Println("No interval set in latest message, setting default from master config:", &defaultInterval)
		message.ReminderIntervals = &defaultInterval
	}

	// Output logging message on message received
	logMessage := fmt.Sprintf(
		"Received message: %s, Channel: %s, Mentions: %v, Reminder Intervals: %d",
		message.Message, message.Channel, message.Mentions, message.ReminderIntervals,
	)
	infoLogger.Println(logMessage)

	go startMonitoring(message, api)
}

func startMonitoring(message SlackMessage, api *slack.Client) {
	if message.Mentions != nil {
		message.Message = makeMentionList(message.Mentions) + " " + message.Message
	}

	parent_message := sendSlackMessage(message.Channel, message.Message, "", api)

	// Setup a timer for the reviewing_internals
	timeCheckInMinutes := message.ReminderIntervals
	ticker := time.NewTicker(time.Duration(*timeCheckInMinutes) * time.Minute)
	for range ticker.C {
		// check if the message has the approval emoji + remove from monitoring if so
		// Check if the message has the reviewing emoji + if so send a bump to the users reviewing
		// if no matching emojis are present, send follow up message

	}
	// sendSlackMessage(message.Channel, message.Message, parent_message, api)
}

func main() {
	config := Config{}
	err := gonfig.GetConf("config.json", &config)
	if err != nil {
		errorLogger.Println("Error loading configuration:", err)
		return
	}

	// Error if the config file is missing required items
	if config.ApprovalEmoji == "" || config.ReviewingEmoji == "" {
		errorLogger.Println("The required fields, approval_emoji or reviewing_emoji is mising")
		return
	}

	// Setup slack authentication
	if config.SlackAccessToken == "" {
		errorLogger.Println("Missing Slack Token, please fix this!")
		return
	}
	api := slack.New(config.SlackAccessToken)

	// Output information on config.json
	infoLogger.Println("Using Approval Emoji:", config.ApprovalEmoji)
	infoLogger.Println("Using Reviewing Emoji:", config.ReviewingEmoji)
	infoLogger.Println("Reviewing Interval:", config.ReviewingInterval)

	// Listen on /message endpoint and pass in default interval from master config
	http.HandleFunc("/message", func(write http.ResponseWriter, request *http.Request) {
		handleMessage(write, request, config.ReviewingInterval, api)
	})
	http.ListenAndServe(":80", nil)
}
