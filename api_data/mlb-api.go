package api_data

// WHATIS: this file has structs for the game data that comes back from the MLB API!
// there are 2 main endpoints used: schedule, which lists all games on a given date
// and live game info, which gives information on a game with a given ID

import (
	"encoding/json"
	"io"
	"time"
)

func (s *Schedule) FromJSON(r io.Reader) error {
	e := json.NewDecoder(r)
	return e.Decode(s)
}

// response to schedule endpoint
type Schedule struct {
	Copyright            string `json:"copyright"`
	TotalItems           int    `json:"totalItems"`
	TotalEvents          int    `json:"totalEvents"`
	TotalGames           int    `json:"totalGames"`
	TotalGamesInProgress int    `json:"totalGamesInProgress"`
	Dates                []Date `json:"dates"`
}
type Status struct {
	AbstractGameState string `json:"abstractGameState"`
	CodedGameState    string `json:"codedGameState"`
	DetailedState     string `json:"detailedState"`
	StatusCode        string `json:"statusCode"`
	StartTimeTBD      bool   `json:"startTimeTBD"`
	AbstractGameCode  string `json:"abstractGameCode"`
}
type LeagueRecord struct {
	Wins   int    `json:"wins"`
	Losses int    `json:"losses"`
	Pct    string `json:"pct"`
}
type TeamName struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}
type Team struct {
	LeagueRecord LeagueRecord `json:"leagueRecord"`
	Score        int          `json:"score"`
	Team         TeamName     `json:"team"`
	IsWinner     bool         `json:"isWinner"`
	SplitSquad   bool         `json:"splitSquad"`
	SeriesNumber int          `json:"seriesNumber"`
}
type Teams struct {
	Away Team `json:"away"`
	Home Team `json:"home"`
}
type Venue struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}
type Content struct {
	Link string `json:"link"`
}
type Game struct {
	GamePk                 uint32    `json:"gamePk"`
	GameGUID               string    `json:"gameGuid"`
	Link                   string    `json:"link"`
	GameType               string    `json:"gameType"`
	Season                 string    `json:"season"`
	GameDate               time.Time `json:"gameDate"`
	OfficialDate           string    `json:"officialDate"`
	Status                 Status    `json:"status"`
	Teams                  Teams     `json:"teams"`
	Venue                  Venue     `json:"venue"`
	Content                Content   `json:"content"`
	IsTie                  bool      `json:"isTie"`
	GameNumber             int       `json:"gameNumber"`
	PublicFacing           bool      `json:"publicFacing"`
	DoubleHeader           string    `json:"doubleHeader"`
	GamedayType            string    `json:"gamedayType"`
	Tiebreaker             string    `json:"tiebreaker"`
	CalendarEventID        string    `json:"calendarEventID"`
	SeasonDisplay          string    `json:"seasonDisplay"`
	DayNight               string    `json:"dayNight"`
	ScheduledInnings       int       `json:"scheduledInnings"`
	ReverseHomeAwayStatus  bool      `json:"reverseHomeAwayStatus"`
	InningBreakLength      int       `json:"inningBreakLength"`
	GamesInSeries          int       `json:"gamesInSeries"`
	SeriesGameNumber       int       `json:"seriesGameNumber"`
	SeriesDescription      string    `json:"seriesDescription"`
	RecordSource           string    `json:"recordSource"`
	IfNecessary            string    `json:"ifNecessary"`
	IfNecessaryDescription string    `json:"ifNecessaryDescription"`
}
type Date struct {
	Date                 string `json:"date"`
	TotalItems           int    `json:"totalItems"`
	TotalEvents          int    `json:"totalEvents"`
	TotalGames           int    `json:"totalGames"`
	TotalGamesInProgress int    `json:"totalGamesInProgress"`
	Games                []Game `json:"games"`
	Events               []any  `json:"events"`
}

// response to live game endpoint
func (lg *LiveGame) FromJSON(r io.Reader) error {
	e := json.NewDecoder(r)
	return e.Decode(lg)
}

type LiveGame struct {
	GamePk   int      `json:"gamePk"`
	GameData GameData `json:"gameData"`
	LiveData LiveData `json:"liveData"`
}
type Datetime struct {
	DateTime     time.Time `json:"dateTime"`
	// OriginalDate string    `json:"originalDate"`
	// OfficialDate string    `json:"officialDate"`
	// DayNight     string    `json:"dayNight"`
}
type Status2 struct {
	AbstractGameState string `json:"abstractGameState"`
	DetailedState     string `json:"detailedState"`
}
type League struct {
	Name string `json:"name"`
}
type Team2 struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	League       League `json:"league"`
}
type Teams2 struct {
	Away Team2 `json:"away"`
	Home Team2 `json:"home"`
}
type PlayerNamed struct {
	ID            uint32 `json:"id"`
	FullName      string `json:"fullName"`
	PrimaryNumber string `json:"primaryNumber"`
}
type Away struct {
	ID       uint32 `json:"id"`
	FullName string `json:"fullName"`
	Link     string `json:"link"`
}
type Home struct {
	ID       uint32 `json:"id"`
	FullName string `json:"fullName"`
	Link     string `json:"link"`
}
type ProbablePitchers struct {
	Away PlayerID `json:"away"`
	Home PlayerID `json:"home"`
}
type GameData struct {
	Datetime         Datetime               `json:"datetime"`
	Status           Status                 `json:"status"`
	Teams            Teams2                 `json:"teams"`
	Players          map[string]PlayerNamed `json:"players"`
	ProbablePitchers ProbablePitchers       `json:"probablePitchers"`
}
type TeamName2 struct {
	Name string `json:"name"`
}
type Defense struct {
	Pitcher PlayerID `json:"pitcher"`
	Team    Team2    `json:"team"`
}
type PlayerID struct {
	ID uint32 `json:"id"`
}
type Offense struct {
	Batter  PlayerID  `json:"batter"`
	First   PlayerID  `json:"first"`
	Second  PlayerID  `json:"second"`
	Third   PlayerID  `json:"third"`
	Pitcher PlayerID  `json:"pitcher"`
	Team    TeamName2 `json:"team"`
}
type Linescore struct {
	CurrentInning uint8   `json:"currentInning"`
	InningHalf    string  `json:"inningHalf"`
	Teams         Teams3  `json:"teams"`
	Defense       Defense `json:"defense"`
	Offense       Offense `json:"offense"`
	Outs          uint8   `json:"outs"`
}
type Decisions struct {
	Winner PlayerID `json:"winner"`
	Loser  PlayerID `json:"loser"`
}
type LiveData struct {
	Linescore Linescore `json:"linescore"`
	Decisions Decisions `json:"decisions"`
}
type Teams3 struct {
	Home Team3 `json:"home"`
	Away Team3 `json:"away"`
}
type Team3 struct {
	Runs uint8 `json:"runs"`
}
