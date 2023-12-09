package botdbStats

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jinzhu/gorm"
	"github.com/xtraice/go-utils/database"
)

var db *gorm.DB
var stats *GoogleTranslateStats
var once sync.Once
var userMonthlyQuota = 1000
var userDailyQuota = 30

type GoogleTranslateStats struct {
	gorm.Model
	ResetDateTime     time.Time `gorm:"type:datetime"`
	LastUserSuccess   time.Time `gorm:"type:datetime"`
	LastUserFailure   time.Time `gorm:"type:datetime"`
	SymbolsTranslated uint64
	SymbolsMonthlyCap uint64
	TranslateSessions []BotTranslateSession
	BlacklistedUsers  []BlacklistedUser
	Servers           []DiscordServer
	Users             []DiscordUser
}

type BlacklistedUser struct {
	gorm.Model
	StartBanDateTime time.Time   `gorm:"type:datetime"`
	EndBanDateTime   time.Time   `gorm:"type:datetime"`
	UserID           uint        `gorm:"unique"` // Add a unique constraint
	User             DiscordUser `gorm:"foreignKey:UserID"`
}

type BotTranslateSession struct {
	gorm.Model
	StartDateTime    time.Time `gorm:"type:datetime"`
	EndDateTime      time.Time `gorm:"type:datetime"`
	PopularLanguage  string
	BlackListedUsers []string `gorm:"type:json"`
}

type DiscordUser struct {
	gorm.Model
	Username         string
	DailyQuota       uint16
	MonthlyQuota     uint16
	NumOfTranslates  uint64
	LastTranslateUse time.Time     `gorm:"type:datetime"`
	LangStats        UserLangStats `gorm:"embedded;embeddedPrefix:lang_stats_"`
}

type DiscordServer struct {
	gorm.Model
	ServerName string
	Members    []DiscordUser `gorm:"foreignKey:UserID"`
}

type UserLangStats struct {
	FromLanguage []string `gorm:"type:json"`
	ToLanguage   []string `gorm:"type:json"`
}

func BotDbConnect() bool {
	home := os.Getenv("HOME")
	if t := database.GetCredentials(path.Join(home, "/go/src/creds.json")); !t {
		return false
	}
	if !database.Connect("discordBot") {
		return false
	}
	db = database.GetDB()
	db.AutoMigrate(&GoogleTranslateStats{}, &BlacklistedUser{}, &BotTranslateSession{}, &DiscordUser{}, &UserLangStats{})
	return true
}

func init() {
	home := os.Getenv("HOME")
	database.GetCredentials(path.Join(home, "/go/src/creds.json"))
	database.Connect("discordBot")
	db = database.GetDB()
	db.AutoMigrate(&GoogleTranslateStats{}, &BlacklistedUser{}, &BotTranslateSession{}, &DiscordUser{}, &UserLangStats{})

	if db := db.Where("ID=?", 1).Find(stats); db.Error != nil {
		TranslateBotStatsInit()
	}
	fmt.Printf("symbols translated: %d\n", stats.SymbolsTranslated)

}

func TranslateBotStatsInit() {
	stats = GetInstance() //singleton
	fmt.Println("INIT Translate Stats Table")
	db.NewRecord(stats)
	db.Create(&stats)
}

// GetInstance returns the singleton instance
func GetInstance() *GoogleTranslateStats {
	once.Do(func() {
		stats = &GoogleTranslateStats{
			Model: gorm.Model{ID: 1},
			ResetDateTime: time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1,
				0, 0, 0, 0, time.Now().UTC().Location()).AddDate(0, 1, 0),
			LastUserSuccess:   time.Now().UTC(),
			LastUserFailure:   time.Now().UTC(),
			SymbolsMonthlyCap: 500000,
			SymbolsTranslated: 1000,
		}
	})
	return stats
}

func UpdateLocalSymbolCnt(s int) {
	stats.SymbolsTranslated += uint64(s)
}

func SaveNow() {
	db.Save(stats)
}

func DiscordUserLangStatUpdate(m discordgo.MessageCreate, langFrom string, langTo string) {
	mId, _ := strconv.Atoi(m.Author.ID)
	has, pUser := stats.containsUser(uint(mId))
	if !has {
		user := DiscordUser{
			Model: gorm.Model{
				ID:        uint(mId),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
			Username:        m.Author.Username,
			NumOfTranslates: 0,
		}
		stats.Users = append(stats.Users, user)
		pUser = &stats.Users[len(stats.Users)-1]
		sort.Slice(stats.Users, func(i, j int) bool {
			return stats.Users[i].ID < stats.Users[j].ID
		})
	}

	pUser.DailyQuota++
	pUser.MonthlyQuota++
	pUser.LastTranslateUse = time.Now().UTC()
	pUser.NumOfTranslates++
	if !slices.Contains(pUser.LangStats.FromLanguage, langFrom) {
		pUser.LangStats.FromLanguage = append(pUser.LangStats.FromLanguage, langFrom)
	}
	if !slices.Contains(pUser.LangStats.ToLanguage, langTo) {
		pUser.LangStats.ToLanguage = append(pUser.LangStats.ToLanguage, langTo)
	}

}

func (*GoogleTranslateStats) containsUser(id uint) (bool, *DiscordUser) {
	n, found := slices.BinarySearchFunc(stats.Users, id, func(a DiscordUser, b uint) int {
		return cmp.Compare(a.ID, id)
	})

	if found {
		return true, &stats.Users[n]
	}

	return false, nil
}

func CheckAndUpdateTranslateReset() {
	for {
		until := time.Until(stats.ResetDateTime)
		if until > 0 {
			time.Sleep(until)
		}
		stats.ResetDateTime = stats.ResetDateTime.AddDate(0, 1, 0)
		newStat := GoogleTranslateStats{ResetDateTime: stats.ResetDateTime}
		db.Model(stats).Update(newStat)
	}
}
