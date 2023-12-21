package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"

	botdbStats "github.com/xtraice/go-discord-bot/pkg/bot_dbstats"
	botTranslate "github.com/xtraice/go-discord-bot/pkg/bot_translate"
	botUtils "github.com/xtraice/go-discord-bot/pkg/bot_utils"

	"github.com/bwmarrin/discordgo"
)

var botContext context.Context
var credentials_ botCredentials

type botCredentials struct {
	ApiKey   string `json:"apikey"`
	BotToken string `json:"bottoken"`
}

// main is the entry point of the program.
// It initializes the Discord bot, sets up event handlers, and starts the bot.
func main() {
	botContext = context.Background()
	home := os.Getenv("HOME")
	if !DiscordGetCredentials(path.Join(home, "/go/src/botCreds.json")) {
		fmt.Println("failed to get discord bot credentials")
		return
	}
	os.Setenv("DISCORD_BOT_TOKEN", credentials_.BotToken)

	dg, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		fmt.Println(err)
		return
	}

	// Launch goroutine to check if Monthly translate stat Resets
	go botdbStats.CheckAndUpdateTranslateReset()
	// Setup Interval Update Stats, currently 30 secs
	go botdbStats.UpdateDBInterval(30000)

	// Initialize Translate Client
	if err := botTranslate.InitTranslateClient(&botContext, credentials_.ApiKey); err != nil {
		fmt.Println("failed to get translate client: ", err)
		return
	}

	//assign callback to set game status when bot is ready
	dg.AddHandler(ready)

	//assign callback for when a message is created in the channel
	dg.AddHandler(messageCreate)

	//assign callback for when a new guild(server) is added
	dg.AddHandler(guildCreate)

	// We need information about guilds (which includes their channels),
	// messages and voice states.
	dg.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsDirectMessages

	fmt.Println("setup discord bot")
	if err := dg.Open(); err != nil {
		fmt.Println("error opening discord session: ", err)
	}

	fmt.Println("Bot now running! Press CTRL+C to exit")

	sc := make(chan os.Signal, 1)

	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	botdbStats.SaveNow()
	dg.Close()
}

func ready(s *discordgo.Session, event *discordgo.Ready) {

	// set the playing status
	s.UpdateGameStatus(0, "Jacked Up & Good To Go")
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	fmt.Printf("Called messageCreate\n")
	//Check if Message is from this bot, if so then return
	if m.Author.ID == s.State.User.ID {
		fmt.Println("Message is from Bot")
		return
	}
	var cmd string
	guildID, _ := strconv.Atoi(m.GuildID)
	authorID, _ := strconv.Atoi(m.Author.ID)
	if m.GuildID != "" {
		botdbStats.AddServer(s, m)
		cmd = botUtils.GetCmd(m.Content)
	} else {
		fmt.Println("Message is from Direct Message")
		cmd = m.Content
	}

	handleCommand(s, m, cmd, guildID, authorID)
}

func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			s.ChannelMessageSend(channel.ID, s.State.User.Username+": Do not fear, I am here!")
		}
	}
}

func DiscordGetCredentials(jsonFile string) bool {
	credentials_ = botCredentials{}
	file, err := os.Open(jsonFile)
	if err != nil {
		fmt.Println("Error opening file, check if path is valid: ", err)
		return false
	}
	defer file.Close()

	// read our opened jsonFile as a byte array.
	byteValue, _ := io.ReadAll(file)

	err = json.Unmarshal(byteValue, &credentials_)
	if err != nil {
		fmt.Println("Failed to unmarshal Credential type data, check json file: ", err)
		return false
	}

	if len(credentials_.ApiKey) <= 0 {
		fmt.Printf("Credentials are empty. credential json incorrect.")
		return false
	}

	return true
}
