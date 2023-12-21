package botdbStats

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xtraice/go-utils/database"
	"gorm.io/gorm"
)

var lock = &sync.Mutex{}
var db *gorm.DB
var stats_ *GoogleTranslateStats
var once sync.Once
var userMonthlyQuota = 1000
var userDailyQuota = 30

var StatsCmds = map[string]func(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int){
	"translate users": handleTranslateUsersCommand,
	"userstats":       handleUserStatsCommand,
}

var StatsHelpCmds = map[string]string{
	"<translate users>": "Get List of all Translating Users in Server",
	"<userstats>":       "Get User Stats for Self '<userstats>' or Another User '<userstats> <username>'",
}

func handleTranslateUsersCommand(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int) {
	fmt.Println("Got 'translate users' Cmd")
	fmt.Printf("Permission needed: %x\n", discordgo.PermissionAdministrator)

	perm, err := s.State.MessagePermissions(m.Message)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("member permissions: %x\n", perm)
	if perm&discordgo.PermissionAdministrator != 0 {
		fmt.Println("Is Admin")
		// This member has admin privileges
		gid, err := strconv.Atoi(m.GuildID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Error Getting Users of this Server")
			return
		}
		guild, _ := s.State.Guild(m.GuildID)
		str := fmt.Sprintf("Server: %s\n\n", guild.Name)
		for _, user := range GetServerUsers(uint(gid)) {
			str += fmt.Sprintf("%s, since: %s\n", user.Username, user.CreatedAt.Format(time.DateOnly))
		}
		s.ChannelMessageSend(m.ChannelID, str)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you don't have the privilege to use that command.")
	}
	fmt.Println("End Cmd")
}

func handleUserStatsCommand(s *discordgo.Session, m *discordgo.MessageCreate, guildID, authorID int) {
	fmt.Println("Got 'get user stats' Cmd")
	fmt.Printf("Permission needed: %x\n", discordgo.PermissionAdministrator)
	var enStr string
	if enStr = strings.TrimLeft(m.Content, "<userstats> "); len(enStr) <= 0 {
		enStr = m.Author.Username
	}

	fmt.Println("User:", enStr)

	var sUser DiscordUser
	for _, user := range GetAllUsers() {
		if user.Username == enStr {
			sUser = user
			fmt.Println("Found User")
			break
		}
	}

	perm, err := s.State.MessagePermissions(m.Message)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("member permissions: %x\n", perm)

	if perm&discordgo.PermissionAdministrator != 0 ||
		sUser.Username == m.Author.Username {

		fmt.Println("Is Admin or User")

		s.ChannelMessageSend(m.ChannelID, FormatUserStats(sUser.ID))
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you don't have the privilege to use that command.")
	}
	fmt.Println("End Cmd")
}

type GoogleTranslateStats struct {
	gorm.Model
	ResetDateTime     time.Time `gorm:"type:datetime"`
	LastUserSuccess   time.Time `gorm:"type:datetime"`
	LastUserFailure   time.Time `gorm:"type:datetime"`
	SymbolsTranslated uint64
	SymbolsMonthlyCap uint64
	TranslateSessions []BotTranslateSession `gorm:"foreignKey:GoogleTranslateStatsID"`
	BlacklistedUsers  []BlacklistedUser     `gorm:"foreignKey:GoogleTranslateStatsID"`
	Servers           []DiscordServer       `gorm:"foreignKey:GoogleTranslateStatsID"`
}

type BlacklistedUser struct {
	gorm.Model
	GoogleTranslateStatsID uint        // Foreign key referencing the ID field from GoogleTranslateStats
	StartBanDateTime       time.Time   `gorm:"type:datetime"`
	EndBanDateTime         time.Time   `gorm:"type:datetime"`
	UserID                 uint        `gorm:"unique"` // Add a unique constraint
	User                   DiscordUser `gorm:"foreignKey:UserID"`
}

type BotTranslateSession struct {
	gorm.Model
	GoogleTranslateStatsID uint      // Foreign key referencing the ID field from GoogleTranslateStats
	StartDateTime          time.Time `gorm:"type:datetime"`
	EndDateTime            time.Time `gorm:"type:datetime"`
	PopularLanguage        string
	BlackListedUsers       []string `gorm:"type:json" json:"blacklisted_users,omitempty"`
}

type DiscordUser struct {
	gorm.Model
	//GoogleTranslateStatsID uint // Foreign key referencing the ID field from GoogleTranslateStats
	DiscordServerID  uint // Foreign key referencing the ID field from DiscordServer
	Username         string
	DailyQuota       uint16
	DailyAccrued     uint16
	MonthlyQuota     uint16
	MonthlyAccrued   uint16
	NumOfTranslates  uint64
	LastTranslateUse time.Time     `gorm:"type:datetime"`
	LangStats        UserLangStats `gorm:"embedded;embeddedPrefix:lang_stats_"`
}

type DiscordServer struct {
	gorm.Model
	GoogleTranslateStatsID uint // Foreign key referencing the ID field from GoogleTranslateStats
	ServerName             string
	Members                []DiscordUser `gorm:"foreignKey:DiscordServerID"`
}

type UserLangStats struct {
	FromLanguage json.RawMessage `gorm:"type:json" json:"lang_stats_from_language,omitempty"`
	ToLanguage   json.RawMessage `gorm:"type:json" json:"lang_stats_to_language,omitempty"`
}

func (uls *UserLangStats) GetFromAsSlice() []string {
	var langs []string
	err := json.Unmarshal(uls.FromLanguage, &langs)
	if err != nil {
		fmt.Errorf("failed to unmarshal FromLanguage: %v", uls.FromLanguage)
		return nil
	}
	return langs
}

func (uls *UserLangStats) GetToAsSlice() []string {
	var langs []string
	err := json.Unmarshal(uls.ToLanguage, &langs)
	if err != nil {
		fmt.Errorf("failed to unmarshal ToLanguage: %v", uls.ToLanguage)
		return nil
	}
	return langs
}

// func for UserLangStats which takes an input slice and sets the FromLanguage field
func (uls *UserLangStats) SetFromLanguage(langs []string) error {
	langJSON, err := json.Marshal(langs)
	if err != nil {
		return fmt.Errorf("failed to marshal langs: %v", langs)
	}
	uls.FromLanguage = langJSON
	return nil
}

// func for UserLangStats which takes an input slice and sets the ToLanguage field
func (uls *UserLangStats) SetToLanguage(langs []string) error {
	langJSON, err := json.Marshal(langs)
	if err != nil {
		return fmt.Errorf("failed to marshal langs: %v", langs)
	}
	uls.ToLanguage = langJSON
	return nil
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
	db.AutoMigrate(&GoogleTranslateStats{}, &BlacklistedUser{}, &BotTranslateSession{}, &UserLangStats{})
	// if dbs := db.Where("ID=?", 1).Find(*stats); dbs.Error != nil {
	stats_ = getInstance() //singleton
	if dbs := db.Preload("BlacklistedUsers").
		Preload("TranslateSessions").
		Preload("Servers").
		Preload("Servers.Members").
		Where("ID=?", 1).
		First(stats_); dbs.Error != nil {
		TranslateBotStatsInit()
	}
	fmt.Printf("symbols translated: %d\n", stats_.SymbolsTranslated)

}

func TranslateBotStatsInit() {
	fmt.Println("INIT Translate Stats Table")
	db.Create(&stats_)
}

// getInstance returns the singleton instance
func getInstance() *GoogleTranslateStats {
	once.Do(func() {
		if stats_ == nil {
			lock.Lock()
			defer lock.Unlock()
			if stats_ == nil {
				fmt.Println("Creating Singleton Instance")
				stats_ = &GoogleTranslateStats{
					Model: gorm.Model{ID: 1},
					ResetDateTime: time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1,
						0, 0, 0, 0, time.Now().UTC().Location()).AddDate(0, 1, 0),
					LastUserSuccess:   time.Now().UTC(),
					LastUserFailure:   time.Now().UTC(),
					SymbolsMonthlyCap: 500000,
					SymbolsTranslated: 0,
				}
			} else {
				fmt.Println("Singleton already created")
			}
		} else {
			fmt.Println("Singleton already created")
		}
	})
	return stats_
}

func UpdateLocalSymbolCnt(s int) {
	stats_.SymbolsTranslated += uint64(s)
	fmt.Printf("UpdateLocalSymbolCnt: %d\n", stats_.SymbolsTranslated)

}

func SaveNow() {
	lock.Lock()
	defer lock.Unlock()
	if d := db.Save(stats_); d.Error != nil {
		fmt.Printf("dbStats::SaveNow():: %s", d.Error.Error())
	}
}

func DiscordUserLangStatUpdate(m discordgo.MessageCreate, langFrom string, langTo string) (bool, error) {
	mId, _ := strconv.Atoi(m.Author.ID)
	has, pUser, pServer := stats_.containsUser(uint(mId))
	fmt.Printf("Msg User: %s\n", m.Author.Username)
	if !has {
		user := DiscordUser{
			Model: gorm.Model{
				ID:        uint(mId),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
			Username:        m.Author.Username,
			NumOfTranslates: 0,
			DailyQuota:      uint16(userDailyQuota),
			MonthlyQuota:    uint16(userMonthlyQuota),
			DailyAccrued:    0,
			MonthlyAccrued:  0,
		}
		gid, _ := strconv.Atoi(m.GuildID)
		n, found := slices.BinarySearchFunc(stats_.Servers, uint(gid), func(a DiscordServer, b uint) int {
			return cmp.Compare(a.ID, b)
		})

		if found {
			pServer = &stats_.Servers[n]
		} else {
			return false, fmt.Errorf("could not find a server associated with user")
		}

		pServer.Members = append(pServer.Members, user)

		//Debug Print
		if has, _, g := stats_.containsUser(uint(mId)); has {
			println("Users is stored in stats_")
			fmt.Printf("%s\n", g.Members[0].Username)
		}

		pUser = &(pServer.Members[len(pServer.Members)-1])
		fmt.Println("pUser\n", pUser)
		sort.Slice(pServer.Members, func(i, j int) bool {
			return pServer.Members[i].ID < pServer.Members[j].ID
		})
		fmt.Println("Added User")
	} else {
		fmt.Println("User already exists")
	}

	langFromJSON, err := json.RawMessage(langFrom).MarshalJSON()
	if err != nil {
		return false, err
	}

	langToJSON, err := json.RawMessage(langTo).MarshalJSON()
	if err != nil {
		return false, err
	}

	fromSlice := pUser.LangStats.GetFromAsSlice()
	if !slices.Contains(fromSlice, langFrom) {
		fromSlice = append(fromSlice, string(langFromJSON))
		pUser.LangStats.SetFromLanguage(fromSlice)
		fmt.Println("fromSlice: ", fromSlice, len(fromSlice))
	}
	toSlice := pUser.LangStats.GetToAsSlice()
	if !slices.Contains(toSlice, langTo) {
		toSlice = append(toSlice, string(langToJSON))
		pUser.LangStats.SetToLanguage(toSlice)
		fmt.Println("toSlice: ", toSlice, len(toSlice))
	}

	pUser.LastTranslateUse = time.Now().UTC()
	pUser.NumOfTranslates += 1
	pUser.DailyAccrued += 1
	pUser.MonthlyAccrued += 1

	if res := db.Session(&gorm.Session{FullSaveAssociations: true}).Save(stats_); res.Error != nil {
		fmt.Printf("dbStats::DiscordUserLangStatUpdate::%s\n", res.Error.Error())
	}

	return true, nil
}

func (*GoogleTranslateStats) containsUser(uid uint) (bool, *DiscordUser, *DiscordServer) {

	for _, s := range stats_.Servers {

		n, found := slices.BinarySearchFunc(s.Members, uid, func(a DiscordUser, b uint) int {
			return cmp.Compare(a.ID, uid)
		})

		if found {
			return true, &s.Members[n], &s
		}
	}
	return false, nil, nil
}

func GetAllUsers() []DiscordUser {
	Users := make([]DiscordUser, 0)
	for _, server := range stats_.Servers {
		Users = append(Users, server.Members...)
	}
	return Users
}

func AddServer(s *discordgo.Session, m *discordgo.MessageCreate) {

	id, _ := strconv.Atoi(m.GuildID)
	mGid := uint(id)
	// Does server already exist
	for _, ds := range stats_.Servers {
		if ds.ID == mGid {
			return
		}
	}
	fmt.Println("Adding Server")
	gName, _ := s.State.Guild(m.GuildID)
	//Create new Server
	stats_.Servers = append(stats_.Servers, DiscordServer{
		Model: gorm.Model{
			ID:        uint(mGid),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		GoogleTranslateStatsID: 1,
		ServerName:             gName.Name,
		Members:                make([]DiscordUser, 0),
	})
	sort.Slice(stats_.Servers, func(i, j int) bool {
		return stats_.Servers[i].ID < stats_.Servers[j].ID
	})
}

func GetServerUsers(guildId uint) []DiscordUser {
	for _, server := range stats_.Servers {
		if server.ID == guildId {
			return server.Members
		}
	}
	return nil
}

func CheckAndUpdateTranslateReset() {
	for {
		until := time.Until(stats_.ResetDateTime)
		if until > 0 {
			time.Sleep(until)
		}
		stats_.ResetDateTime = stats_.ResetDateTime.AddDate(0, 1, 0)
		stats_.SymbolsTranslated = 0
		newStat := GoogleTranslateStats{
			ResetDateTime:     stats_.ResetDateTime,
			SymbolsTranslated: stats_.SymbolsTranslated}

		db.Model(stats_).Updates(newStat)
	}
}

func UpdateDBInterval(intervalMSecs int64) {
	now := time.Now().UTC().UnixMilli()
	start := now
	for {
		now := time.Now().UTC().UnixMilli()
		if res := now - start; res > intervalMSecs {
			SaveNow()
			start = now
			fmt.Println("Saved Stats (Interval)")
		}
	}
}

// A function to check if a user is blacklisted
func IsBlacklisted(uid uint) bool {
	for _, u := range stats_.BlacklistedUsers {
		if u.UserID == uid {
			return true
		}
	}
	return false
}

// A function to add a user to the blacklist
func AddToBlacklist(uid uint) {
	stats_.BlacklistedUsers = append(stats_.BlacklistedUsers, BlacklistedUser{
		Model: gorm.Model{
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		GoogleTranslateStatsID: 1,
		UserID:                 uid,
		StartBanDateTime:       time.Now().UTC(),
		EndBanDateTime:         time.Now().UTC().AddDate(0, 0, 7),
	})
}

// A function to remove a user from the blacklist
func RemoveFromBlacklist(uid uint) {
	for i, u := range stats_.BlacklistedUsers {
		if u.UserID == uid {
			stats_.BlacklistedUsers = append(stats_.BlacklistedUsers[:i], stats_.BlacklistedUsers[i+1:]...)
		}
	}
}

// ResetDailyQuota resets the daily quota for a user identified by their UID.
// It iterates through all the servers and members in the stats_ variable and checks if the user ID matches.
// If the user's last translation usage is not from the current day, their daily accrued count is reset to 0.
// @param uid: The user's ID
func ResetDailyQuota(uid uint) {
	for _, s := range stats_.Servers {
		for _, u := range s.Members {
			if u.ID == uid {
				if u.LastTranslateUse.Day() != time.Now().UTC().Day() {
					u.DailyAccrued = 0
				}
			}
		}
	}
}

// A function to format a user's stats as a string so each stat can be displayed on a new line
func FormatUserStats(uid uint) string {
	var str string
	for _, s := range stats_.Servers {
		for _, u := range s.Members {
			if u.ID == uid {
				str += fmt.Sprintf("User: %s\n", u.Username)
				str += fmt.Sprintf("Daily Quota: %d\n", u.DailyQuota)
				str += fmt.Sprintf("Daily Accrued: %d\n", u.DailyAccrued)
				str += fmt.Sprintf("Monthly Quota: %d\n", u.MonthlyQuota)
				str += fmt.Sprintf("Monthly Accrued: %d\n", u.MonthlyAccrued)
				str += fmt.Sprintf("Number of Translates: %d\n", u.NumOfTranslates)
				str += fmt.Sprintf("Last Translate Use: %s\n", u.LastTranslateUse.Format(time.RFC1123))
				str += fmt.Sprintf("From Language: %v\n", u.LangStats.GetFromAsSlice())
				str += fmt.Sprintf("To Language: %v\n", u.LangStats.GetToAsSlice())
			}
		}
	}
	return str
}

// A bool func which receives a serverId and userId,
// then returns whether a user has exceeded their daily or monthly quota.
// @param serverId: The server's ID
// @param userId: The user's ID
// @return: A bool indicating whether the user has exceeded their daily quota
func ExceedsQuotaOrBanned(serverId uint, userId uint) bool {
	n, found := slices.BinarySearchFunc(stats_.Servers, serverId, func(a DiscordServer, b uint) int {
		return cmp.Compare(a.ID, b)
	})
	if !found {
		return true
	} else {
		server := stats_.Servers[n]
		n, found := slices.BinarySearchFunc(server.Members, userId, func(a DiscordUser, b uint) int {
			return cmp.Compare(a.ID, b)
		})
		if !found {
			return true
		} else {
			user := server.Members[n]
			is := IsBlacklisted(userId) ||
				user.MonthlyAccrued >= user.MonthlyQuota ||
				user.DailyAccrued >= user.DailyQuota
			return is
		}
	}
}
