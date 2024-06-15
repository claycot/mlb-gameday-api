package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/api_data"
)

type Games struct {
	Metadata Metadata `json:"metadata"`
	Data     []*Game  `json:"data"`
}

type Metadata struct {
	Timestamp string `json:"timestamp"`
}

type Game struct {
	ID    uint32 `json:"id"`
	State State  `json:"state"`
	Teams Teams  `json:"teams"`
}

type State struct {
	Inning  Inning  `json:"inning"`
	Diamond Diamond `json:"diamond"`
	Outs    uint8   `json:"outs"`
	Status  Status  `json:"status"`
}

type Inning struct {
	Number     uint8  `json:"number"`
	Top_bottom string `json:"top_bottom"`
}

type Diamond struct {
	Batter Player `json:"batter"`
	First  Player `json:"first"`
	Second Player `json:"second"`
	Third  Player `json:"third"`
}

type Status struct {
	General   string            `json:"general"`
	Detailed  string            `json:"detailed"`
	StartTime api_data.Datetime `json:"start_time"`
}

type Time struct {
	Display      string `json:"display"`
	DateTime     string `json:"dateTime"`
	OriginalDate string `json:"originalDate"`
	OfficialDate string `json:"officialDate"`
	DayNight     string `json:"dayNight"`
	ShortTime    string `json:"time"`
	AmPm         string `json:"ampm"`
}

type Teams struct {
	Away Team `json:"away"`
	Home Team `json:"home"`
}

type Team struct {
	Info    Info   `json:"info"`
	Pitcher Player `json:"pitcher"`
	Score   uint8  `json:"score"`
}

type Info struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	League       string `json:"league"`
}

type Player struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

func (g *Games) ToJSON() ([]byte, error) {
	js, err := json.Marshal(g)
	return js, err
}

// create a store for game information, so if 100000 users are online (standard load)
// we only get 1 set of requests to the MLB API
var (
	gamesStore = struct {
		sync.RWMutex
		data Games // this isn't a pointer to games because games would be created as a temp object, and we need this to persist!
	}{}
)

// wrap GetGames in a cache mechanism that invalidates data older than 30 seconds!
func GetCachedGames(dateString string) (*Games, error) {
	// read the cached games object, which includes a metadata timestamp and games
	gamesStore.RLock()
	gamesObj := gamesStore.data
	gamesStore.RUnlock()

	// attempt to parse timestamp from the gamesObj metadata
	ts, err := time.Parse(time.RFC3339Nano, gamesObj.Metadata.Timestamp)

	// if we couldn't parse the timestamp, or if it's older than 30 seconds, refresh data!
	if err != nil || time.Since(ts) > 30*time.Second {
		gamesStore.Lock()

		// force a refresh
		gPtr, err := GetGames(dateString)
		if err != nil {
			return nil, err
		}

		// write the data to the cache
		gamesStore.data = *gPtr

		gamesStore.Unlock()
	}

	// return the information on the games inside of the gamesStore
	return &gamesStore.data, nil
}

// get formatted information on live games with a given date string MM/DD/YYYY (or "" to get today)
func GetGames(dateString string) (*Games, error) {
	// set the date for the game fetch
	if dateString == "" {
		// force LA time since server might change day early
		pacificTime, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			fmt.Println("err: ", err.Error())
		}
		dateString = time.Now().In(pacificTime).Format("01/02/2006")
	}

	apiUrl := fmt.Sprintf("%s/api/v1/schedule/?sportId=1&date=%s", os.Getenv("MLB_API_URL"), dateString)
	fmt.Println(apiUrl)

	// get the list of today's games from MLB
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}

	// marshal the list of games into a struct
	schedule := api_data.Schedule{}
	err = schedule.FromJSON(resp.Body)
	if err != nil {
		return nil, err
	} else if len(schedule.Dates) == 0 {
		return nil, fmt.Errorf("schedule endpoint returned no games for provided date: %s", dateString)
	}

	games := &Games{
		Metadata: Metadata{
			Timestamp: time.Now().Format(time.RFC3339Nano),
		},
		Data: make([]*Game, len(schedule.Dates[0].Games)),
	}

	// process requested games from Dates[0]
	var wg sync.WaitGroup
	for gameNum := 0; gameNum < len(schedule.Dates[0].Games); gameNum++ {
		// each game will require a request to a different API endpoint for live game info (pitchers, score, etc.)
		wg.Add(1)
		go func(gameIndex int) {
			// wait until the game is processed
			defer wg.Done()

			// get information on the live game, from the link provided in the schedule response
			// fmt.Printf("dispatching request for game %d at link %s\n", gameIndex, schedule.Dates[0].Games[gameIndex].Link)
			resp, err := http.Get(os.Getenv("MLB_API_URL") + schedule.Dates[0].Games[gameIndex].Link)
			if err != nil {
				fmt.Printf("error: %e", err)
			}
			// fmt.Printf("got game info for game %d; status %s\n", gameIndex, resp.Status)

			// marshal the live game data into a struct
			lg := api_data.LiveGame{}
			err = lg.FromJSON(resp.Body)
			if err != nil {
				fmt.Printf("error: %e", err)
			}

			// make a map of the players in the game, starting with a null value
			players := make(map[uint32]*Player)
			players[0] = &Player{
				ID:     0,
				Name:   "TBD",
				Number: "-1",
			}

			// add each player in the game into the players map
			for _, p := range lg.GameData.Players {
				players[p.ID] = &Player{
					ID:     p.ID,
					Name:   p.FullName,
					Number: p.PrimaryNumber,
				}
			}

			// set pitcher information based on game state
			var pitcherHomeID uint32
			var pitcherAwayID uint32
			switch lg.GameData.Status.AbstractGameState {
			case "Preview":
				pitcherAwayID = lg.GameData.ProbablePitchers.Away.ID
				pitcherHomeID = lg.GameData.ProbablePitchers.Home.ID
			case "Live":
				if lg.GameData.Teams.Away.Name == lg.LiveData.Linescore.Defense.Team.Name {
					pitcherAwayID = lg.LiveData.Linescore.Defense.Pitcher.ID
					pitcherHomeID = lg.LiveData.Linescore.Offense.Pitcher.ID
				} else {
					pitcherAwayID = lg.LiveData.Linescore.Offense.Pitcher.ID
					pitcherHomeID = lg.LiveData.Linescore.Defense.Pitcher.ID
				}
			case "Final":
				if lg.LiveData.Linescore.Teams.Away.Runs > lg.LiveData.Linescore.Teams.Home.Runs {
					pitcherAwayID = lg.LiveData.Decisions.Winner.ID
					pitcherHomeID = lg.LiveData.Decisions.Loser.ID
				} else {
					pitcherAwayID = lg.LiveData.Decisions.Loser.ID
					pitcherHomeID = lg.LiveData.Decisions.Winner.ID
				}
			}

			// set information about teams
			th := &Team{
				Info: Info{
					Name:         lg.GameData.Teams.Home.Name,
					Abbreviation: lg.GameData.Teams.Home.Abbreviation,
					League:       lg.GameData.Teams.Home.League.Name,
				},
				Pitcher: *players[pitcherHomeID],
				Score:   lg.LiveData.Linescore.Teams.Home.Runs,
			}
			ta := &Team{
				Info: Info{
					Name:         lg.GameData.Teams.Away.Name,
					Abbreviation: lg.GameData.Teams.Away.Abbreviation,
					League:       lg.GameData.Teams.Away.League.Name,
				},
				Pitcher: *players[pitcherAwayID],
				Score:   lg.LiveData.Linescore.Teams.Away.Runs,
			}
			t := &Teams{
				Away: *ta,
				Home: *th,
			}

			// set information about the game state
			s := &State{
				Inning: Inning{
					Number:     lg.LiveData.Linescore.CurrentInning,
					Top_bottom: lg.LiveData.Linescore.InningHalf,
				},
				Diamond: Diamond{
					Batter: *players[lg.LiveData.Linescore.Offense.Batter.ID],
					First:  *players[lg.LiveData.Linescore.Offense.First.ID],
					Second: *players[lg.LiveData.Linescore.Offense.Second.ID],
					Third:  *players[lg.LiveData.Linescore.Offense.Third.ID],
				},
				Outs: lg.LiveData.Linescore.Outs,
				Status: Status{
					General:   lg.GameData.Status.AbstractGameState,
					Detailed:  lg.GameData.Status.DetailedState,
					StartTime: lg.GameData.Datetime,
				},
			}

			// catch API quirks in batter display
			// 1. they're batting and also on base
			// 2. if there are 3 outs, the team is still at bat but the other team's batter is up
			if s.Outs == 3 ||
				s.Diamond.Batter == s.Diamond.First ||
				s.Diamond.Batter == s.Diamond.Second ||
				s.Diamond.Batter == s.Diamond.Third {
				s.Diamond.Batter = *players[0]
			}

			// update information for finalized games
			if s.Status.General == "Final" {
				// clear the batter
				s.Diamond.Batter = *players[0]
				// zero the outs
				s.Outs = 0
			}

			// write information to the return object
			// fmt.Printf("writing game data for %d\n", gameIndex)
			// fmt.Printf("data: %v", lg)
			games.Data[gameIndex] = &Game{
				ID:    schedule.Dates[0].Games[gameIndex].GamePk,
				State: *s,
				Teams: *t,
			}
		}(gameNum)
	}

	// wait until all game data has been retrieved
	wg.Wait()

	// sort the list of games
	sortGames(games.Data)

	// fmt.Println("returning")
	// fmt.Printf("games: %v", games)
	return games, nil
}

// sort games in-place
func sortGames(games []*Game) {
	statusOrder := map[string]int{
		"Live":    0,
		"Final":   1,
		"Preview": 2,
	}

	sort.Slice(games, func(i, j int) bool {
		g1, g2 := games[i], games[j]

		statusComp := statusOrder[g1.State.Status.General] - statusOrder[g2.State.Status.General]
		if statusComp != 0 {
			return statusComp < 0
		}

		// disabled because it causes games to jump around as they outpace others
		// // if both games are live, sort by inning number (higher first)
		// if g1.State.Status.General == "Live" {
		// 	return g2.State.Inning.Number < g1.State.Inning.Number
		// }

		// otherwise, sort by start time (earliest first)
		g1Start := g1.State.Status.StartTime.DateTime
		g2Start := g2.State.Status.StartTime.DateTime

		return g1Start.Before(g2Start)
	})
}
