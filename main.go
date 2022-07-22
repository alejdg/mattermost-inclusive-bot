package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/apps/appclient"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
	"github.com/mattermost/mattermost-server/v5/model"
)

//go:embed icon.png
var iconData []byte

//go:embed manifest.json
var manifestData []byte

//go:embed bindings.json
var bindingsData []byte

//go:embed send_form.json
var formData []byte

const (
	BOT_NAME           = "inclusive-bot"
	BOT_TOKEN          = "9gw8jfjcm3y8ugb19sr4xcaube"
	CHANNEL_NAME       = "general"
	TEAM_NAME          = "test"
	DEBUG_CHANNEL_NAME = "debug-" + BOT_NAME
)

var client *model.Client4
var webSocketClient *model.WebSocketClient

var botUser *model.User
var botTeam *model.Team
var debuggingChannel *model.Channel

func main() {
	// Serve its own manifest as HTTP for convenience in dev. mode.
	http.HandleFunc("/manifest.json", httputils.DoHandleJSONData(manifestData))

	// Returns the Channel Header and Command bindings for the app.
	http.HandleFunc("/bindings", httputils.DoHandleJSONData(bindingsData))

	// The form for sending a Hello message.
	http.HandleFunc("/send/form", httputils.DoHandleJSONData(formData))

	// The main handler for sending a Hello message.
	http.HandleFunc("/send/submit", send)

	// Forces the send form to be displayed as a modal.
	http.HandleFunc("/send-modal/submit", httputils.DoHandleJSONData(formData))

	// Serves the icon for the app.
	http.HandleFunc("/static/icon.png",
		httputils.DoHandleData("image/png", iconData))

	setupBot()

	WebSocketHandling()
	addr := ":4000" // matches manifest.json
	fmt.Println("Listening on", addr)
	fmt.Println("/apps install http http://dune.local" + addr + "/manifest.json") // matches manifest.json
	log.Fatal(http.ListenAndServe(addr, nil))

}

func setupBot() {
	client = model.NewAPIv4Client("http://localhost:8066")
	client.SetToken(BOT_TOKEN)
	GetTeam()
	GetBotUser()
	GetDebugChannel()
}

func send(w http.ResponseWriter, req *http.Request) {
	c := apps.CallRequest{}
	json.NewDecoder(req.Body).Decode(&c)

	message := "Hello, world!"
	v, ok := c.Values["message"]
	if ok && v != nil {
		message += fmt.Sprintf(" ...and %s!", v)
	}
	appclient.AsBot(c.Context).DM(c.Context.ActingUserID, message)

	httputils.WriteJSON(w,
		apps.NewDataResponse("Created a post in your DM channel."))
}

func WebSocketHandling() {
	// Lets start listening to some channels via the websocket!
	webSocketClient, err := model.NewWebSocketClient4("ws://localhost:8066", BOT_TOKEN)
	if err != nil {
		println("We failed to connect to the web socket")
		PrintError(err)
	}
	println("websocket sequence", webSocketClient.Sequence)

	println("API URL:")
	println(webSocketClient.ApiUrl)
	println("AUTH Token:")
	println(webSocketClient.AuthToken)
	webSocketClient.Listen()

	go func() {
		for resp := range webSocketClient.EventChannel {
			HandleWebSocketResponse(resp)
			webSocketClient.SendMessage("user_typing", make(map[string]interface{}))
		}
	}()

	// // You can block forever with
	// select {}
}

func HandleWebSocketResponse(event *model.WebSocketEvent) {
	HandleMsgFromDebuggingChannel(event)
}

func HandleMsgFromDebuggingChannel(event *model.WebSocketEvent) {
	fmt.Printf("%+v\n", event)
	fmt.Printf("%+v\n", debuggingChannel.Id)

	// If this isn't the debugging channel then lets ingore it
	if event.Broadcast.ChannelId != debuggingChannel.Id {
		return
	}

	fmt.Printf("%+v\n", event)

	// Lets only respond to messaged posted events
	if event.Event != model.WEBSOCKET_EVENT_POSTED {
		return
	}

	println("responding to debugging channel msg")

	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post != nil {

		// ignore my events
		if post.UserId == botUser.Id {
			return
		}

		//alejdg
		term := "slave"
		replacement := "secondary"
		_ = replacement
		if matched, _ := regexp.MatchString(term, post.Message); matched {
			println("\t UserId: " + post.UserId)
			println("\t post.Id: " + post.Id)
			SendEphemeralMsgToUser("You're using outdate terms my friend! Here are some alternatives for it:", post.Id, post.UserId)
			// ReplaceTerm(term, replacement, post)
			return
		}

		// if you see any word matching 'alive' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)alive(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'up' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)up(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'running' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)running(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}

		// if you see any word matching 'hello' then respond
		if matched, _ := regexp.MatchString(`(?:^|\W)hello(?:$|\W)`, post.Message); matched {
			SendMsgToDebuggingChannel("Yes I'm running", post.Id)
			return
		}
	}

	SendMsgToDebuggingChannel("I did not understand you!", post.Id)
}

func PrintError(err *model.AppError) {
	println("\tError Details:")
	println("\t\t" + err.Message)
	println("\t\t" + err.Id)
	println("\t\t" + err.DetailedError)
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

func SendEphemeralMsgToUser(msg string, replyToId string, userId string) {
	attachments := []model.SlackAttachment{}
	actions := []*model.PostAction{}
	words := []string{"secondary", "agent"}
	for _, word := range words {
		action := model.PostAction{
			Id:   word,
			Name: word,
			Integration: &model.PostActionIntegration{
				Context: map[string]interface{}{
					"URL":    "ws://localhost:8065/",
					"action": "do_something_ephemeral",
				},
			},
		}
		actions = append(actions, &action)
	}
	attachment := model.SlackAttachment{}
	attachment.Actions = actions
	attachments = append(attachments, attachment)
	post := &model.Post{}
	// post := &model.Post{
	// 	Props: map[string]interface{}{
	// 		"attachments": []model.SlackAttachment{
	// 			attachment,
	// 		},
	// 	},
	// }
	postEphemeral := &model.PostEphemeral{}
	post.ChannelId = debuggingChannel.Id
	post.Message = msg

	_ = attachment
	_ = actions

	post.RootId = replyToId
	postEphemeral.UserID = userId
	postEphemeral.Post = post
	postEphemeral.Post.AddProp("attachments", attachments)
	println(postEphemeral.Post.Attachments())
	println(postEphemeral.Post)
	fmt.Printf("%+v\n", postEphemeral.Post)
	println("ALEJDG TEST:")
	webSocketClient.SendMessage("get_status", nil)
	if _, resp := client.CreatePostEphemeral(postEphemeral); resp.Error != nil {
		println("We failed to send a message to the logging channel")
		PrintError(resp.Error)
	}
}

func ReplaceTerm(term string, replacement string, post *model.Post) {
	re := regexp.MustCompile(term)
	post.Message = re.ReplaceAllString(post.Message, replacement)

	// post.RootId = replyToId

	if _, resp := client.UpdatePost(post.Id, post); resp.Error != nil {
		println("We failed to update the message")
		PrintError(resp.Error)
	}
}

func GetTeam() {
	if team, resp := client.GetTeamByName(TEAM_NAME, ""); resp.Error != nil {
		println("We failed to get the initial load")
		println("or we do not appear to be a member of the team '" + TEAM_NAME + "'")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botTeam = team
	}
}

func GetDebugChannel() {
	if rchannel, resp := client.GetChannelByName(DEBUG_CHANNEL_NAME, botTeam.Id, ""); resp.Error != nil {
		println("We failed to get the channels")
		PrintError(resp.Error)
	} else {
		debuggingChannel = rchannel
		return
	}
	CreateBotDebuggingChannel()
}

func GetBotUser() {
	if user, resp := client.GetUserByUsername(BOT_NAME, ""); resp.Error != nil {
		println("There was a problem logging into the Mattermost server.  Are you sure ran the setup steps from the README.md?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		botUser = user
	}
}

func CreateBotDebuggingChannel() {
	channel := &model.Channel{}
	channel.Name = DEBUG_CHANNEL_NAME
	channel.DisplayName = "Debugging For Sample Bot"
	channel.Purpose = "This is used as a test channel for logging bot debug messages"
	channel.Type = model.CHANNEL_OPEN
	channel.TeamId = botTeam.Id
	if rchannel, resp := client.CreateChannel(channel); resp.Error != nil {
		println("We failed to create the channel " + DEBUG_CHANNEL_NAME)
		PrintError(resp.Error)
	} else {
		debuggingChannel = rchannel
		println("Looks like this might be the first run so we've created the channel " + DEBUG_CHANNEL_NAME)
	}
}

// func SetupGracefulShutdown() {
// 	c := make(chan os.Signal, 1)
// 	signal.Notify(c, os.Interrupt)
// 	go func() {
// 		for _ = range c {
// 			if webSocketClient != nil {
// 				webSocketClient.Close()
// 			}

// 			SendMsgToDebuggingChannel("_"+SAMPLE_NAME+" has **stopped** running_", "")
// 			os.Exit(0)
// 		}
// 	}()
// }
