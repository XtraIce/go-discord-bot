package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	botdbStats "github.com/xtraice/go-discord-bot/pkg/bot_dbstats"
	botTranslate "github.com/xtraice/go-discord-bot/pkg/bot_translate"
)

var CmdCenter = []*map[string]func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int){
	&commands,
	&botTranslate.TranslateCmds,
	&botdbStats.StatsCmds,
}

var commands = map[string]func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int){
	"help": handleHelpCommand,
}

var helpCmds = []*map[string]string{
	{"<help>": "Get Help"},
	&botTranslate.TranslateHelpCmds,
	&botdbStats.StatsHelpCmds,
}

// func transfroms helpCmds into a string
func helpCmdsToString() string {
	var helpStr string
	for _, helpMap := range helpCmds {
		for cmd, desc := range *helpMap {
			helpStr += fmt.Sprintf("%s: %s\n", cmd, desc)
		}
	}
	return helpStr
}

func handleCommand(s *discordgo.Session, m *discordgo.MessageCreate, cmd string, guildID, authorID int) {
	fmt.Println("Got Cmd:", cmd)
	for _, commandMap := range CmdCenter {
		if commandFunc, ok := (*commandMap)[cmd]; ok {
			commandFunc(s, m, guildID, authorID)
			return
		}
	}
}

func handleHelpCommand(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int) {
	s.ChannelMessageSend(m.ChannelID, helpCmdsToString())
}
