package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/api_data"
)

type Games struct {
	Metadata Metadata `json:"metadata"`
	Data     []*Game  `json:"data"`
}

type GameCache struct {
	cache sync.Map
}

type Game struct {
	Metadata Metadata `json:"metadata"`
	Link     string   `json:"link"`
	ID       uint32   `json:"id"`
	State    State    `json:"state"`
	Teams    Teams    `json:"teams"`
}

type Metadata struct {
	Timestamp time.Time `json:"timestamp"`
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

// Set updates or adds a game to the cache
func (gc *GameCache) Set(id uint32, g Game) {
	gc.cache.Store(id, g)
}

// Get retrieves a game from the cache
func (gc *GameCache) Get(id uint32) (Game, bool) {
	if value, ok := gc.cache.Load(id); ok {
		return value.(Game), true
	}
	return Game{}, false
}

// Delete removes a game from the cache
func (gc *GameCache) Delete(id uint32) {
	gc.cache.Delete(id)
}

// Fetch updates the cache with new game data
func (gc *GameCache) Fetch(link string, wg *sync.WaitGroup) (bool, error) {
	// get updated information on the game
	newGame, err := FetchGame(link, wg)

	// if failed to fetch, return err
	if err != nil {
		return false, err
	}

	// if successful, check if the game has changed
	oldGame, exists := gc.Get(newGame.ID)
	if !exists || !reflect.DeepEqual(oldGame, newGame) {
		gc.Set(newGame.ID, newGame)
		return true, nil
	}
	return false, nil
}

// Audit cache (refresh games and prune dead games)
func (gc *GameCache) Audit(wg *sync.WaitGroup) ([]uint32, []uint32, []uint32) {
	var updated, removed, failed []uint32
	gc.cache.Range(func(key, value interface{}) bool {
		game := value.(Game)
		id := key.(uint32)

		if game.State.Status.General == "Live" && time.Since(game.Metadata.Timestamp) > (30*time.Second) {
			// refresh active games
			dataChanged, err := gc.Fetch(game.Link, wg)
			if err != nil {
				failed = append(failed, id)
			} else if dataChanged {
				updated = append(updated, id)
			}
		} else if game.State.Status.General == "Final" && time.Since(game.Metadata.Timestamp) > (12*time.Hour) {
			// prune games that have been over for 12 hours
			gc.Delete(id)
			removed = append(removed, id)
		}
		return true
	})
	return updated, removed, failed
}

// wrap GetGames in a cache mechanism that invalidates data older than 30 seconds!
func GetInitialGames(gamesStore *GameCache) (*Games, error) {
	gameIds, gameLinks, err := ListGamesByDate("04/25/2024")
	if err != nil {
		return nil, err
	}

	// load all of today's games from the cache
	games := make([]*Game, len(gameIds))
	var wg sync.WaitGroup
	for i := 0; i < len(gameIds); i++ {
		game, valid := gamesStore.Get(gameIds[i])
		if !valid {
			fmt.Printf("fetching information on game %d at link %s\n", gameIds[i], gameLinks[i])
			updated, err := gamesStore.Fetch(gameLinks[i], &wg)
			if err == nil && updated {
				game, valid = gamesStore.Get(gameIds[i])
			}
		}

		if valid {
			fmt.Printf("getting cached information on game %d\n", gameIds[i])
			games[i] = &game
		} else {
			return nil, fmt.Errorf("failed to fetch information on game: %d", gameIds[i])
		}
	}

	// wait until all game data has been retrieved
	wg.Wait()

	// sort the list of games
	sortGames(games)

	// fmt.Println("returning")
	// fmt.Printf("games: %v", games)
	return &Games{
		Metadata: Metadata{
			Timestamp: time.Now(),
		},
		Data: games,
	}, nil
}

// get formatted information on live games with a given date string MM/DD/YYYY (or "" to get today)
func ListGamesByDate(dateString string) ([]uint32, []string, error) {
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
		return nil, nil, err
	}
	defer resp.Body.Close()

	// marshal the list of games into a struct
	schedule := api_data.Schedule{}
	err = schedule.FromJSON(resp.Body)
	if err != nil {
		return nil, nil, err
	} else if len(schedule.Dates) == 0 {
		return nil, nil, fmt.Errorf("schedule endpoint returned no games for provided date: %s", dateString)
	}

	gameIds := make([]uint32, len(schedule.Dates[0].Games))
	gameLinks := make([]string, len(schedule.Dates[0].Games))
	for gameNum := 0; gameNum < len(schedule.Dates[0].Games); gameNum++ {
		gameIds[gameNum] = schedule.Dates[0].Games[gameNum].GamePk
		gameLinks[gameNum] = os.Getenv("MLB_API_URL") + schedule.Dates[0].Games[gameNum].Link
	}
	return gameIds, gameLinks, nil
}

// get game object given an ID
func FetchGame(link string, wg *sync.WaitGroup) (Game, error) {
	// wait until the game is processed
	wg.Add(1)
	defer wg.Done()

	// get information on the live game, from the link provided in the schedule response
	// fmt.Printf("dispatching request for game %d at link %s\n", gameIndex, schedule.Dates[0].Games[gameIndex].Link)
	resp, err := http.Get(link)
	if err != nil {
		fmt.Printf("error: %e", err)
		return Game{}, err
	}
	// fmt.Printf("got game info for game %d; status %s\n", gameIndex, resp.Status)

	// marshal the live game data into a struct
	lg := api_data.LiveGame{}
	err = lg.FromJSON(resp.Body)
	if err != nil {
		fmt.Printf("error: %e", err)
		return Game{}, err
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
	// 1. if the game hasn't started
	// 2. if there are 3 outs, the team is still at bat but the other team's batter is up
	// 3. they're batting and also on base
	if s.Status.General != "Live" ||
		s.Outs == 3 ||
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
	return Game{
		ID:    uint32(lg.GamePk),
		Link:  link,
		State: *s,
		Teams: *t,
		Metadata: Metadata{
			Timestamp: time.Now(),
		},
	}, nil
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
