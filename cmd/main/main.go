package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	botdbStats "github.com/xtraice/go-discord-bot/pkg/bot_dbstats"
	botTranslate "github.com/xtraice/go-discord-bot/pkg/bot_translate"
	botUtils "github.com/xtraice/go-discord-bot/pkg/bot_utils"

	"cloud.google.com/go/translate"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
)

var botContext context.Context
var availableCmds = [...]string{"jpen", "enjp"}

var credentials_ botCredentials

type botCredentials struct {
	ApiKey   string `json:"apikey"`
	BotToken string `json:"bottoken"`
}

var gClient_ *translate.Client

func main() {
	botContext = context.Background()

	if !DiscordGetCredentials("/go/src/creds.json") {
		fmt.Println("failed to get discord bot credentials")
		return
	}
	os.Setenv("DISCORD_BOT_TOKEN", credentials_.BotToken)

	dg, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		fmt.Println(err)
		return
	}

	if t := botdbStats.BotDbConnect(); !t {
		fmt.Println("Failed to Connect to database")
		os.Exit(1)
	}

	// Launch goroutine to check if Monthly translate stat Resets
	go botdbStats.CheckAndUpdateTranslateReset()

	fmt.Println("Setup new client.")
	gClient_, err = translate.NewClient(botContext, option.WithAPIKey(credentials_.ApiKey))
	if err != nil {
		fmt.Println("failed to get translate client: ", err)
	}
	defer gClient_.Close()

	//assign callback to set game status when bot is ready
	dg.AddHandler(ready)

	//assign callback for when a message is created in the channel
	dg.AddHandler(messageCreate)

	//assign callback for when a new guild(server) is added
	dg.AddHandler(guildCreate)

	// We need information about guilds (which includes their channels),
	// messages and voice states.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates
	fmt.Println("setup discord bot")
	if err := dg.Open(); err != nil {
		fmt.Println("error opening discord session: ", err)
	}

	fmt.Println("Bot now running! Press CTRL+C to exit")

	sc := make(chan os.Signal, 1)

	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

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

	cmd := botUtils.GetCmd(m.Content)

	switch cmd {
	case "help":
		cmds := "jpen - Japanese To English \n enjp - English To Japanese \n envi - English to Vietnamese \n vien - Vietnamese to English \n"
		s.ChannelMessageSend(m.ChannelID, cmds)

	case "jpen":
		fmt.Println("Got jpen Cmd")
		if jpStr := strings.TrimLeft(m.Content, "<jpen>"); len(jpStr) > 0 {
			strSlice := []string{jpStr}
			respStr, _ := botTranslate.Translate(gClient_, botContext, strSlice, language.Japanese, language.English)

			s.ChannelMessageSend(m.ChannelID, respStr)
			botdbStats.DiscordUserLangStatUpdate(*m, language.Japanese.String(), language.English.String())
			return
		}
	case "enjp":
		fmt.Println(("Got enjp Cmd"))
		if enStr := strings.TrimLeft(m.Content, "<enjp>"); len(enStr) > 0 {
			strSlice := []string{enStr}
			respStr, _ := botTranslate.Translate(gClient_, botContext, strSlice, language.English, language.Japanese)

			s.ChannelMessageSend(m.ChannelID, respStr)
			botdbStats.DiscordUserLangStatUpdate(*m, language.English.String(), language.Japanese.String())
			return
		}
	case "vien":
		fmt.Println(("Got vien Cmd"))
		if enStr := strings.TrimLeft(m.Content, "<vien>"); len(enStr) > 0 {
			strSlice := []string{enStr}
			respStr, _ := botTranslate.Translate(gClient_, botContext, strSlice, language.Vietnamese, language.English)

			s.ChannelMessageSend(m.ChannelID, respStr)
			botdbStats.DiscordUserLangStatUpdate(*m, language.Vietnamese.String(), language.English.String())
			return
		}
	case "envi":
		fmt.Println(("Got envi Cmd"))
		if enStr := strings.TrimLeft(m.Content, "<envi>"); len(enStr) > 0 {
			strSlice := []string{enStr}
			respStr, _ := botTranslate.Translate(gClient_, botContext, strSlice, language.English, language.Vietnamese)

			s.ChannelMessageSend(m.ChannelID, respStr)
			botdbStats.DiscordUserLangStatUpdate(*m, language.English.String(), language.Vietnamese.String())
			return
		}
	case "koen":
		fmt.Println(("Got envi Cmd"))
		if enStr := strings.TrimLeft(m.Content, "<koen>"); len(enStr) > 0 {
			strSlice := []string{enStr}
			respStr, _ := botTranslate.Translate(gClient_, botContext, strSlice, language.Korean, language.English)

			s.ChannelMessageSend(m.ChannelID, respStr)
			botdbStats.DiscordUserLangStatUpdate(*m, language.Korean.String(), language.English.String())
			return
		}
	case "enko":
		fmt.Println(("Got envi Cmd"))
		if enStr := strings.TrimLeft(m.Content, "<enkor>"); len(enStr) > 0 {
			strSlice := []string{enStr}
			respStr, _ := botTranslate.Translate(gClient_, botContext, strSlice, language.English, language.Korean)

			s.ChannelMessageSend(m.ChannelID, respStr)
			botdbStats.DiscordUserLangStatUpdate(*m, language.English.String(), language.Vietnamese.String())
			return
		}
	}
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
