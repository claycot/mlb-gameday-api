package data

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/api_data"
)

type Games struct {
	Metadata Metadata `json:"metadata"`
	Data     []*Game  `json:"data"`
}

type GameIDs struct {
	Metadata Metadata  `json:"metadata"`
	Data     []*uint32 `json:"data"`
}

type GameCache struct {
	cache  sync.Map
	length uint8
}

type Game struct {
	Metadata Metadata `json:"metadata"`
	Link     string   `json:"link"`
	ID       uint32   `json:"id"`
	State    State    `json:"state"`
}

type Metadata struct {
	Timestamp time.Time `json:"timestamp"`
	Ready     bool      `json:"ready"`
}

type State struct {
	Teams   Teams   `json:"teams"`
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

func (g *GameIDs) ToJSON() ([]byte, error) {
	js, err := json.Marshal(g)
	return js, err
}

// add a partial game to the cache
func (gc *GameCache) Discover(id uint32, link string) (bool, error) {
	// check if the game already exists before discovering
	_, exists := gc.cache.Load(id)
	if exists {
		return false, nil
	}

	// TODO: Magic number uint8 max
	if gc.length == 255 {
		return false, fmt.Errorf("games cache is full with %d games", gc.length)
	}

	// if the game doesn't exist, discover it
	gc.cache.Store(id, Game{
		Metadata: Metadata{
			Timestamp: time.Now(),
			Ready:     false,
		},
		Link: link,
		ID:   id,
	})
	gc.length++

	return true, nil
}

// return the link for a game, or an error if it doesn't exist
func (gc *GameCache) GetLink(id uint32) (string, error) {
	// check if the game exists
	game, exists := gc.cache.Load(id)
	if !exists {
		return "", fmt.Errorf("game with id %d does not exist, even as a partial", id)
	}

	// if the game exists, return the link
	return game.(Game).Link, nil
}

// use the stored game link to update cache game info
func (gc *GameCache) Fetch(ctx context.Context, id uint32) (bool, error) {
	// get the link from the game cache
	link, err := gc.GetLink(id)
	if err != nil {
		return false, err
	}

	// get updated information on the game, passing context to handle cancellation
	newGame, err := FetchGame(ctx, link)
	if err != nil {
		return false, err
	}

	// if successful, check if the game has changed
	oldGameRaw, exists := gc.cache.Load(id)
	if exists {
		oldGame := oldGameRaw.(Game)
		// if the game did not change, return false
		if reflect.DeepEqual(oldGame, newGame) {
			return false, nil
		}
	}

	// otherwise, store the game and return true
	gc.cache.Store(id, newGame)
	return true, nil
}

// retrieve a game from the cache by ID
func (gc *GameCache) GetOne(ctx context.Context, id uint32) (Game, bool) {
	gameRaw, exists := gc.cache.Load(id)

	// if the game doesn't exist, return empty and false
	if !exists {
		return Game{}, false
	}

	game := gameRaw.(Game)

	// if the game exists, but isn't ready, load it
	if !game.Metadata.Ready {
		updated, err := gc.Fetch(ctx, id)

		// if failed to fetch a new version, return empty and false
		if !updated || err != nil {
			return Game{}, false
		}

		// try to load again
		if updatedGameRaw, ok := gc.cache.Load(id); ok {
			return updatedGameRaw.(Game), true
		}

		// if the new load fails, return false
		return Game{}, false
	}

	// if the game was already ready, return it
	return game, true
}

// retrieve all ready games from the cache
func (gc *GameCache) GetAll() ([]*Game, error) {
	if gc.length > 0 {
		games := make([]*Game, gc.length)

		g := 0
		gc.cache.Range(func(key, value interface{}) bool {
			game := value.(Game)

			if game.Metadata.Ready {
				games[g] = &game
				g++
			}
			return true
		})
		return games[0:g], nil
	} else {
		return nil, nil
	}
}

// remove a game from the cache
func (gc *GameCache) Delete(id uint32) {
	_, exists := gc.cache.Load(id)

	// must check if the game exists before decrementing the length
	if exists {
		gc.cache.Delete(id)
		gc.length--
	}
}

// refresh games and prune dead games
func (gc *GameCache) Audit(ctx context.Context) ([]uint32, []uint32, []uint32) {
	var updated, removed, failed []uint32
	gc.cache.Range(func(key, value interface{}) bool {
		game := value.(Game)
		id := key.(uint32)

		// refresh live games
		// also refresh preview and final games (less frequently)
		if (game.State.Status.General == "Live" && time.Since(game.Metadata.Timestamp) > (5*time.Second)) ||
			(game.State.Status.General == "Preview" && time.Since(game.Metadata.Timestamp) > (15*time.Minute)) ||
			(game.State.Status.General == "Final" && time.Since(game.Metadata.Timestamp) > (30*time.Minute)) {
			// refresh active games
			dataChanged, err := gc.Fetch(ctx, id)
			if err != nil {
				failed = append(failed, id)
			} else if dataChanged {
				updated = append(updated, id)
			}
			// prune games that are final and started over 15 hours ago
			// also prune games that don't start for 24 hours (postponed)
		} else if (game.State.Status.General == "Final" && time.Since(game.State.Status.StartTime.DateTime) > (15*time.Hour)) ||
			(game.State.Status.General == "Preview" && time.Until(game.Metadata.Timestamp) > (24*time.Hour)) {
			gc.Delete(id)
			removed = append(removed, id)
		}
		return true
	})
	return updated, removed, failed
}

// when a user first visits, get all games
func GetInitialGames(gamesStore *GameCache) (*Games, error) {
	games, err := gamesStore.GetAll()

	if err != nil {
		return nil, err
	}

	sortGames(games)

	return &Games{
		Metadata: Metadata{
			Timestamp: time.Now(),
			Ready:     true,
		},
		Data: games,
	}, nil
}

// get formatted information on live games with a given date string MM/DD/YYYY (or "" to get today)
func ListGamesByDate(ctx context.Context, logger *log.Logger, dateString string) ([]uint32, []string, error) {
	// set the date for the game fetch
	if dateString == "" {
		// force LA time since server might change day early
		pacificTime, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			return nil, nil, err
		}
		dateString = time.Now().In(pacificTime).Format("01/02/2006")
	}

	// get fields from struct
	fieldsSchedule := generateFieldsString(api_data.Schedule{})

	apiUrl := fmt.Sprintf("%s/api/v1/schedule/?sportId=1&date=%s&fields=%s", os.Getenv("MLB_API_URL"), dateString, fieldsSchedule)

	// log request
	logger.Printf("[INFO] Making request: %s", apiUrl)

	// limit each fetch to 10 seconds
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return nil, nil, err
	}

	// get the list of today's games from MLB
	resp, err := http.DefaultClient.Do(req)
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

	// get fields for links
	fieldsLivegame := generateFieldsString(api_data.LiveGame{})

	gameIds := make([]uint32, len(schedule.Dates[0].Games))
	gameLinks := make([]string, len(schedule.Dates[0].Games))
	for gameNum := range schedule.Dates[0].Games {
		gameIds[gameNum] = schedule.Dates[0].Games[gameNum].GamePk
		// build the link with the desired fields
		gameLinks[gameNum] = fmt.Sprintf("%s%s?fields=%s", os.Getenv("MLB_API_URL"), schedule.Dates[0].Games[gameNum].Link, fieldsLivegame)
	}
	return gameIds, gameLinks, nil
}

// get game object given a link
func FetchGame(ctx context.Context, link string) (Game, error) {
	// get information on the live game, from the link provided in the schedule response
	// fmt.Printf("dispatching request for game %d at link %s\n", gameIndex, schedule.Dates[0].Games[gameIndex].Link)

	// limit each fetch to 10 seconds
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, link, nil)
	if err != nil {
		return Game{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Game{}, err
	}
	defer resp.Body.Close()
	// fmt.Printf("got game info for game %d; status %s\n", gameIndex, resp.Status)

	// marshal the live game data into a struct
	lg := api_data.LiveGame{}
	err = lg.FromJSON(resp.Body)
	if err != nil {
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

	// set information about the game state
	s := &State{
		Teams: Teams{
			Away: *ta,
			Home: *th,
		},
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
		Metadata: Metadata{
			Timestamp: time.Now(),
			Ready:     true,
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

// recursive function to extract field names using JSON tags
func extractFieldsFromStruct(t reflect.Type, prefix string) []string {
	var fields []string

	// get the underlying type from a pointer, map, slice, or array
	if t.Kind() == reflect.Ptr || t.Kind() == reflect.Map || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}

	// if not a struct, return
	if t.Kind() != reflect.Struct {
		return fields
	}

	// iterate over the fields of a struct
	for i := range t.NumField() {
		field := t.Field(i)

		// skip private fields
		if field.PkgPath != "" {
			continue
		}

		// get JSON field tag
		jsonTag := field.Tag.Get("json")

		// skip fields without json tags
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// handle cases where the tag might have options (e.g., "name,omitempty")
		jsonName := strings.Split(jsonTag, ",")[0]

		// construct the full path for nested fields
		fullPath := jsonName
		if prefix != "" {
			fullPath = prefix + "," + jsonName
		}

		// handle different field types
		if field.Type == reflect.TypeOf(time.Time{}) {
			// edge case to handle time as a basic value
			fields = append(fields, fullPath)
		} else if field.Type.Kind() == reflect.Struct || field.Type.Kind() == reflect.Ptr || field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			// recursively extract fields from a nested struct
			fields = append(fields, extractFieldsFromStruct(field.Type, fullPath)...)
		} else {
			// otherwise, just add the field name
			fields = append(fields, fullPath)
		}
	}

	return fields
}

// generate a csv string representing a struct's fields (including nesting)
func generateFieldsString(obj any) string {
	// get the type
	t := reflect.TypeOf(obj)

	// get nested fields from the type
	fields := extractFieldsFromStruct(t, "")

	// join and return fields
	return strings.Join(fields, ",")
}
