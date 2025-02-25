package reaction

import (
	"embed"
	_ "embed"
	"errors"
	"net/http"
	"strconv"
)

var Game = reactionTest{}

type reactionTest struct{}

//go:embed index.html
var content embed.FS

func (g *reactionTest) ShortName() string {
	return "reaction"
}

func (g *reactionTest) FS() *embed.FS {
	return &content
}

func (g *reactionTest) ScoreCalculator(r *http.Request) (int, error) {
	timeStr := r.FormValue("time")
	reactionTime, err := strconv.Atoi(timeStr)
	if err != nil {
		return 0, errors.New("invalid time")
	}

	if reactionTime <= 0 {
		return 0, errors.New("invalid time")
	}
	if reactionTime >= 1000 {
		reactionTime = 1000
	}
	return 1000 - reactionTime, nil
}
