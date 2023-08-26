package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/tkanos/gonfig"
)

var (
	infoLogger      = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	warningLogger   = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime)
	errorLogger     = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	debugLogger     = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	checkinMessages = []string{"How's this going", "What's the latest", "Any update", "Don't forget about me"}
)

type Config struct {
	Port              int    `json:"port"`
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

func randomString(stringList []string) string {
	rand.Seed(time.Now().Unix())
	randomIndex := rand.Intn(len(stringList))
	randomString := stringList[randomIndex]
	return randomString
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
		errorLogger.Fatalf("Error sending message: %v", err)
	}
	return messageTs
}

func retrieveReactions(messageTs, channel string, api *slack.Client) map[string][]string {
	emojiUsersMap := make(map[string][]string)

	// Get all emojis
	emojiList, err := api.GetEmoji()
	if err != nil {
		errorLogger.Fatalf("Error retrieving emoji list: %v", err)
	}

	for emoji := range emojiList {
		emojiUsersMap[emoji] = nil
	}

	reactions, err := api.GetReactions(slack.ItemRef{Channel: channel, Timestamp: messageTs}, slack.GetReactionsParameters{Full: true})
	if err != nil {
		errorLogger.Fatalf("Error retrieving reactions: %v", err)
	}

	// Populate the map with users for each emoji
	for _, reaction := range reactions {
		emojiName := reaction.Name
		emojiUsersMap[emojiName] = reaction.Users
	}
	return emojiUsersMap
}

func handleMessage(write http.ResponseWriter, request *http.Request, config Config, api *slack.Client) {
	decoder := json.NewDecoder(request.Body)
	var message SlackMessage
	err := decoder.Decode(&message)
	if err != nil {
		errorLogger.Printf("Error decoding JSON: %v", err)
	}

	// Set default interval if message is missing this field
	if message.ReminderIntervals == nil {
		debugLogger.Println("No interval set in latest message, setting default from master config:", &config.ReviewingInterval)
		message.ReminderIntervals = &config.ReviewingInterval
	}

	// Output logging message on message received
	logMessage := fmt.Sprintf(
		"Received message: %s, Channel: %s, Mentions: %v, Reminder Intervals: %d",
		message.Message, message.Channel, message.Mentions, message.ReminderIntervals,
	)
	infoLogger.Println(logMessage)

	stopChannel := make(chan struct{})
	go startMonitoring(message, config, api, stopChannel)
}

func startMonitoring(message SlackMessage, config Config, api *slack.Client, stopChannel chan struct{}) {
	// Create mention list if provided in payload
	if message.Mentions != nil {
		message.Message = makeMentionList(message.Mentions) + " " + message.Message
	}
	channel := message.Channel
	mentions := message.Mentions

	// Send parent message
	parent_message := sendSlackMessage(channel, message.Message, "", api)

	// Setup prefix for messages
	prefix := fmt.Sprintf("[%s - %s]", channel, parent_message)

	//Log info message on first message
	infoLogger.Println(prefix, "Sending Initial Slack Message to", channel, ". Parent Message:", parent_message)

	// Setup a timer for the reviewing_internals
	timeCheckInMinutes := message.ReminderIntervals
	ticker := time.NewTicker(time.Duration(*timeCheckInMinutes) * time.Minute)

	// ticker := time.NewTicker(1 * time.Minute)

	for range ticker.C {
		// Grab the reactions from the parent message
		reactions := retrieveReactions(parent_message, channel, api)

		// Log current approvers and reviewers
		debugLogger.Println(prefix, "Approvers:", reactions[config.ApprovalEmoji])
		debugLogger.Println(prefix, "Reviewers:", reactions[config.ReviewingEmoji])

		// check if the message has the approval emoji + remove from monitoring if so
		if reactions[config.ApprovalEmoji] != nil {
			// Remove from loop
			close(stopChannel)

			// Log the message has been resolved
			infoLogger.Println(prefix, "Message has been marked as resolved. Removing from loop")

			// Send message in slack thread to confirm
			approvers := makeMentionList(reactions[config.ApprovalEmoji])
			message := fmt.Sprintf("This message has been resolved by %s", approvers)
			sendSlackMessage(channel, message, parent_message, api)
			break
		}

		if reactions[config.ReviewingEmoji] != nil {
			// Message is being reviewed
			reviewers := makeMentionList(reactions[config.ReviewingEmoji])
			message := fmt.Sprintf("%s %s?", randomString(checkinMessages), reviewers)
			sendSlackMessage(channel, message, parent_message, api)
			continue
		}

		// Message has not been reviewed or approved
		message := fmt.Sprintf("%s This is not currently being reviewed or has not been approved yet!", makeMentionList(mentions))
		sendSlackMessage(channel, message, parent_message, api)

	}
}

func main() {
	config := Config{}
	err := gonfig.GetConf("config.json", &config)
	if err != nil {
		errorLogger.Fatalf("Error loading configuration: %v", err)
	}

	// Error if the config file is missing required items
	if config.ApprovalEmoji == "" || config.ReviewingEmoji == "" || config.SlackAccessToken == "" {
		errorLogger.Fatalf("The required fields, slack_access_token, approval_emoji or reviewing_emoji is mising")
	}

	// Setup slack authentication
	api := slack.New(config.SlackAccessToken)

	// Output information on config.json
	infoLogger.Printf("Using Approval Emoji: %s", config.ApprovalEmoji)
	infoLogger.Printf("Using Reviewing Emoji: %s", config.ReviewingEmoji)
	infoLogger.Printf("Reviewing Interval: %d", config.ReviewingInterval)

	// Listen on /message endpoint and pass in default interval from master config
	http.HandleFunc("/message", func(write http.ResponseWriter, request *http.Request) {
		handleMessage(write, request, config, api)
	})
	if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil); err != nil {
		errorLogger.Fatal(err)
	}
}
