package data

import (
	"testing"

	"github.com/claycot/mlb-gameday-api/api_data"
	"github.com/stretchr/testify/assert"
)

// generate a csv string representing a struct's fields (including nesting)
func TestGenerateFieldsStringSchedule(t *testing.T) {
	expected := "dates,games,gamePk,dates,games,link"

	actual := generateFieldsString(api_data.Schedule{})

	assert.Equal(t, expected, actual, "fields should be correct for schedule endpoint")
}

func TestGenerateFieldsStringLiveGame(t *testing.T) {
	expected := "gamePk,gameData,datetime,dateTime,gameData,status,abstractGameState,gameData,status,detailedState,gameData,teams,away,name,gameData,teams,away,abbreviation,gameData,teams,away,league,name,gameData,teams,home,name,gameData,teams,home,abbreviation,gameData,teams,home,league,name,gameData,players,id,gameData,players,fullName,gameData,players,primaryNumber,gameData,probablePitchers,away,id,gameData,probablePitchers,home,id,liveData,linescore,currentInning,liveData,linescore,inningHalf,liveData,linescore,teams,home,runs,liveData,linescore,teams,away,runs,liveData,linescore,defense,pitcher,id,liveData,linescore,defense,team,name,liveData,linescore,defense,team,abbreviation,liveData,linescore,defense,team,league,name,liveData,linescore,offense,batter,id,liveData,linescore,offense,first,id,liveData,linescore,offense,second,id,liveData,linescore,offense,third,id,liveData,linescore,offense,pitcher,id,liveData,linescore,offense,team,name,liveData,linescore,outs,liveData,decisions,winner,id,liveData,decisions,loser,id"

	actual := generateFieldsString(api_data.LiveGame{})

	assert.Equal(t, expected, actual, "fields should be correct for livegame endpoint")
}
