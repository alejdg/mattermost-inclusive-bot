package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/apps/appclient"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/spf13/viper"
)

//go:embed icon.png
var iconData []byte

//go:embed word_list.json
var wordListData []byte

const (
	BOT_NAME       = "inclusive-bot"
	WORD_LIST_FILE = "word_list.json"
)

var client *model.Client4
var webSocketClient *model.WebSocketClient

var botUser *model.User
var botTeam *model.Team
var debuggingChannel *model.Channel
var wordList map[string]interface{}

func main() {
	SetupGracefulShutdown()

	// The main handler for sending a Hello message.
	http.HandleFunc("/send/submit", send)

	// Replaces a term
	// http.HandleFunc("/replace/", replaceHandler)

	// Serves the icon for the app.
	http.HandleFunc("/static/icon. png",
		httputils.DoHandleData("image/png", iconData))

	setupBot()

	WebSocketHandling()
	addr := ":4000" // matches manifest.json
	fmt.Println("The bot is running")
	// The bot is not requiring installation atm.
	// fmt.Println("Listening on", addr)
	// fmt.Println("/apps install http http://dune.local" + addr + "/manifest.json") // matches manifest.json
	log.Fatal(http.ListenAndServe(addr, nil))

}

func setupBot() {
	GetConfig()
	client = model.NewAPIv4Client(viper.GetString("site_url"))
	client.SetToken(viper.GetString("bot_token"))
	GetTeam()
	GetBotUser()
	GetDebugChannel()
	wordList = GetWordList()
}

func send(w http.ResponseWriter, req *http.Request) {
	c := apps.CallRequest{}
	err := json.NewDecoder(req.Body).Decode(&c)
	if err != nil {
		log.Fatal("Error decoding the request:", err)
	}

	message := "Hello, world!"
	v, ok := c.Values["message"]
	if ok && v != nil {
		message += fmt.Sprintf(" ...and %s!", v)
	}
	_, err = appclient.AsBot(c.Context).DM(c.Context.ActingUserID, message)
	if err != nil {
		log.Fatal("Error sending DM:", err)
	}

	err = httputils.WriteJSON(w,
		apps.NewDataResponse("Created a post in your DM channel."))
	if err != nil {
		log.Fatal("Error writing json:", err)
	}

}

type ReplyData struct {
	AppToken  string `json:"app_token"`
	ReplyToId string `json:"replyToId"`
	Term      string `json:"term"`
	Word      string `json:"word"`
}

// // Requires admin rights. Not in use atm.
// func replaceHandler(w http.ResponseWriter, req *http.Request) {
// 	request := model.PostActionIntegrationRequestFromJson(req.Body)
// 	fmt.Printf("PostActionIntegrationRequestFromJson: \n%s\n", request.ToJson())
// 	fmt.Printf("request.Context: \n%s\n", request.Context)
// 	// TODO validateToken()
// 	var result ReplyData
// 	json.Unmarshal([]byte(request.Context["body"].(string)), &result)
// 	fmt.Printf("result: \n%s\n", result)
// 	post, resp := client.GetPost(result.ReplyToId, "")
// 	if resp.StatusCode == 404 {
// 		println("Couldn't replace the term. Message not found")
// 		return
// 	}
// 	fmt.Printf("post message: \n%s\n", post.Message)
// 	ReplaceTerm(result.Term, result.Word, post)
// }

func WebSocketHandling() {
	// Lets start listening to some channels via the websocket!
	re := regexp.MustCompile(`^[a-z][a-z0-9+\-.]*:`)
	wsa := re.ReplaceAllString(viper.GetString("site_url"), "ws://")
	fmt.Printf("websocket address: \n%s\n", wsa)
	webSocketClient, err := model.NewWebSocketClient4(wsa, viper.GetString("bot_token"))
	if err != nil {
		println("We failed to connect to the web socket")
		PrintError(err)
	}

	webSocketClient.Listen()

	go func() {
		for resp := range webSocketClient.EventChannel {
			HandleWebSocketResponse(resp)
		}
	}()

	SendMsgToDebuggingChannel("I'm online.", "")
}

func HandleWebSocketResponse(event *model.WebSocketEvent) {
	HandleMsg(event)
}

func checkMsg(msg string) (term string, suggestions interface{}, err error) {
	for word, replacements := range wordList {
		if matched, _ := regexp.MatchString(word, msg); matched {
			suggestions = replacements
			term = word
			return
		}
	}
	err = errors.New("no terms found")
	return
}

func HandleMsg(event *model.WebSocketEvent) {

	// Lets only respond to messaged posted events
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {

		// ignore my events
		if post.UserId == botUser.Id {
			return
		}

		// Handle bot dms as debugging messages
		dm_channel := CreateDmChannel(post.UserId)
		if event.Broadcast.ChannelId == dm_channel.Id {
			HandleDirectMsg(event, post)
		}
		term, replacements, err := checkMsg(post.Message)
		if err != nil {
			// println("\t Message okay")
		} else {
			// println("\t UserId: " + post.UserId)
			// println("\t post.Id: " + post.Id)
			post_link := fmt.Sprintf("%s/%s/pl/%s", viper.GetString("site_url"), viper.Get("team_name"), post.Id)
			msg := fmt.Sprintf("You're using outdated terms. Consider using one of the suggestions below. \n**Term**: %s \n**Suggestions**: %v\n**Post**: %s", term, replacements, post_link)
			SendPrivateMessage(msg, post.UserId)
			return
		}
	}
}

func HandleDirectMsg(event *model.WebSocketEvent, post *model.Post) {
	// if you see any word matching 'alive', 'up', 'running', 'hello' then respond
	if matched, _ := regexp.MatchString(`(?:^|\W)(alive|up|running|hello)(?:$|\W)`, post.Message); matched {
		SendPrivateMessage("Yes, I'm running", post.UserId)
		return
	}
}

func PrintError(err *model.AppError) {
	println("\tError Details:")
	println("\t\t" + err.Message)
	println("\t\t" + err.Id)
	println("\t\t" + err.DetailedError)
}

// Requires admin rights. Not in use atm.
// func SendEphemeralMsg(post *model.Post) *model.Response {
// 	postEphemeral := &model.PostEphemeral{}
// 	postEphemeral.Post = post
// 	fmt.Printf("%+v\n", postEphemeral.UserID)
// 	_, resp := client.CreatePostEphemeral(postEphemeral)
// 	if resp.Error != nil {
// 		println("Failed to send an ephemeral messsage.")
// 		PrintError(resp.Error)
// 	}
// 	return resp

// }

func SendMsg(post *model.Post) *model.Response {
	_, resp := client.CreatePost(post)
	if resp.Error != nil {
		println("We failed to send a message to the logging channel")
		PrintError(resp.Error)
	}
	return resp

}

func SendMsgToDebuggingChannel(msg string, replyToId string) {
	post := &model.Post{}
	post.ChannelId = debuggingChannel.Id
	post.Message = msg

	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		println("We failed to send a message to the logging channel")
		PrintError(resp.Error)
	}
}

func SendSuggestion(msg string, replyToId string, userId string, term string) {

}

func SendPrivateMessage(msg string, userId string) {
	post := &model.Post{}
	post.Message = msg
	post.UserId = userId
	channel := CreateDmChannel(userId)
	post.ChannelId = channel.Id
	SendMsg(post)
}

func CreateDmChannel(userId string) (channel *model.Channel) {
	channel, resp := client.CreateDirectChannel(botUser.Id, userId)
	if resp.Error != nil {
		return
	}
	return
}

// Requires admin rights. Not in use atm.
func ReplaceTerm(term string, replacement string, post *model.Post) {
	re := regexp.MustCompile(term)
	post.Message = re.ReplaceAllString(post.Message, replacement)

	if _, resp := client.UpdatePost(post.Id, post); resp.Error != nil {
		println("We failed to update the message")
		PrintError(resp.Error)
		if resp.Error.Id == "api.context.permissions.app_error" {
			println("Make sure the bot is an admin or is in a team with `edit_others_posts` permission (Enterprise only).")
		}
	}
}

func GetTeam() {
	if team, resp := client.GetTeamByName(viper.GetString("team_name"), ""); resp.Error != nil {
		println("We failed to get the initial load")
		println("or we do not appear to be a member of the team '" + viper.GetString("team_name") + "'")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botTeam = team
	}
}

func GetDebugChannel() {
	if rchannel, resp := client.GetChannelByName(viper.GetString("debug_channel_name"), botTeam.Id, ""); resp.Error != nil {
		println("We failed to get the channels")
		PrintError(resp.Error)
	} else {
		debuggingChannel = rchannel
		return
	}
	CreateBotDebuggingChannel()
}

func GetBotUser() {
	if user, resp := client.GetUserByUsername(viper.GetString("bot_name"), ""); resp.Error != nil {
		println("There was a problem logging into the Mattermost server.  Are you sure ran the setup steps from the README.md?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botUser = user
	}
}

func CreateBotDebuggingChannel() {
	channel := &model.Channel{}
	channel.Name = viper.GetString("debug_channel_name")
	channel.DisplayName = "Debugging For Sample Bot"
	channel.Purpose = "This is used as a test channel for logging bot debug messages"
	channel.Type = model.CHANNEL_OPEN
	channel.TeamId = botTeam.Id
	if rchannel, resp := client.CreateChannel(channel); resp.Error != nil {
		println("We failed to create the channel " + viper.GetString("debug_channel_name"))
		PrintError(resp.Error)
	} else {
		debuggingChannel = rchannel
		println("Looks like this might be the first run so we've created the channel " + viper.GetString("debug_channel_name"))
	}
}

func GetWordList() (words map[string]interface{}) {
	content, err := os.ReadFile(WORD_LIST_FILE)
	if err != nil {
		log.Fatal("Error when opening file: ", err)
	}

	err = json.Unmarshal(content, &words)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
	}

	err = json.Unmarshal(wordListData, &words)
	if err != nil {
		log.Fatal("Error during Unmarshal2(): ", err)
	}
	return
}

func GetConfig() {
	viper.AddConfigPath("./")
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("Error loading configuration:", err)
	}
	// TODO: validate required config values
	viper.SetDefault("bot_name", BOT_NAME)
	viper.SetDefault("debug_channel_name", "debug-"+BOT_NAME)
	viper.SetDefault("word_list_file", WORD_LIST_FILE)
}

func SetupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			if webSocketClient != nil {
				webSocketClient.Close()
			}

			SendMsgToDebuggingChannel("I **stopped** running.", "")
			os.Exit(0)
		}
	}()
}
