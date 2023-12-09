package botTranslate

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/translate"
	botdbStats "github.com/xtraice/go-discord-bot/pkg/bot_dbstats"
	"golang.org/x/text/language"
)

var AvailableLangs = [...]string{"en", "jp", "ko", "vi"}

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
