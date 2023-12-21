package botTranslate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/translate"
	"github.com/bwmarrin/discordgo"
	botdbStats "github.com/xtraice/go-discord-bot/pkg/bot_dbstats"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
)

var gClient_ *translate.Client
var botContext_ *context.Context

var TranslateCmds = map[string]func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int){
	"jpen": handleTranslateCommand(language.Japanese, language.English),
	"enjp": handleTranslateCommand(language.English, language.Japanese),
	"vien": handleTranslateCommand(language.Vietnamese, language.English),
	"envi": handleTranslateCommand(language.English, language.Vietnamese),
	"koen": handleTranslateCommand(language.Korean, language.English),
	"enko": handleTranslateCommand(language.English, language.Korean),
	"spen": handleTranslateCommand(language.Spanish, language.English),
	"ensp": handleTranslateCommand(language.English, language.Spanish),
}

var TranslateHelpCmds = map[string]string{
	"<jpen>": "Translate Japanese to English",
	"<enjp>": "Translate English to Japanese",
	"<vien>": "Translate Vietnamese to English",
	"<envi>": "Translate English to Vietnamese",
	"<koen>": "Translate Korean to English",
	"<enko>": "Translate English to Korean",
	"<spen>": "Translate Spanish to English",
	"<ensp>": "Translate English to Spanish",
}

// InitTranslateClient initializes the translation client with the provided API key.
// It returns an error if the client fails to initialize.
//
// @param botContext: The context for the bot.
// @param apiKey: The API key for the translation service.
// @return error: An error if the client fails to initialize.
func InitTranslateClient(botContext *context.Context, apiKey string) error {
	fmt.Println("Initializing Translate Client")
	botContext_ = botContext
	var err error
	gClient_, err = translate.NewClient(*botContext_, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Println("failed to get translate client: ", err)
		return err
	}
	defer gClient_.Close()
	return nil
}

// handleTranslateCommand handles the translation command from one language to another.
// It takes the source language and target language as parameters and returns a function
// that can be executed to perform the translation.
//
// @param fromLang: The source language.
// @param toLang: The target language.
// @return func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int): The translation command handler function.
func handleTranslateCommand(fromLang, toLang language.Tag) func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int) {
		fmt.Println("Got", fromLang, "Cmd")
		if enStr := strings.TrimLeft(m.Content, "<"+fromLang.String()+">"); len(enStr) > 0 {
			strSlice := []string{enStr}
			if !botdbStats.ExceedsQuotaOrBanned(uint(guildID), uint(authorID)) {
				if botContext_ == nil {
					fmt.Println("botContext_ is nil")
					return
				}
				respStr, _ := Translate(gClient_, *botContext_, strSlice, fromLang, toLang)

				s.ChannelMessageSend(m.ChannelID, respStr)
				botdbStats.DiscordUserLangStatUpdate(*m, fromLang.String(), toLang.String())
			}
			return
		}
	}
}

// Translate performs the translation using the provided translation client and context.
// It takes the English strings to be translated, the source language tag, and the target language tag as parameters.
// It returns the translated string and an error if the translation fails.
//
// @param gClient: The translation client.
// @param ctx: The context for the translation.
// @param engStrs: The English strings to be translated.
// @param srcTag: The source language tag.
// @param tgtTag: The target language tag.
// @return string: The translated string.
// @return error: An error if the translation fails.
func Translate(gClient *translate.Client, ctx context.Context,
	engStrs []string, srcTag language.Tag, tgtTag language.Tag) (string, error) {
	if gClient == nil {
		return "", errors.New("google client not initialized")
	}

	resps, err := gClient.Translate(ctx, engStrs, tgtTag,
		&translate.Options{
			Source: srcTag,
			Format: translate.Text,
		})
	if err != nil {
		fmt.Println("Failed to translate, error: ", err)
		return "Error Translating, please make sure the input language is Japanese", err
	}

	//put all strings together
	var finalString string
	charCnt := 0
	for _, t := range resps {
		finalString += t.Text + "\n"
		charCnt += len(t.Text)
	}
	botdbStats.UpdateLocalSymbolCnt(charCnt)

	return finalString, nil
}
