package clicks

import (
	"embed"
	"errors"
	"net/http"
	"strconv"
)

var Game = reactionTest{}

type reactionTest struct{}

//go:embed index.html
var content embed.FS

func (g *reactionTest) ShortName() string {
	return "clicks"
}

func (g *reactionTest) FS() *embed.FS {
	return &content
}

func (g *reactionTest) ScoreCalculator(r *http.Request) (int, error) {
	clicksStr := r.FormValue("clicks")
	clicks, err := strconv.Atoi(clicksStr)
	if err != nil {
		return 0, errors.New("invalid clicks")
	}

	if clicks <= 0 {
		return 0, errors.New("invalid clicks")
	}
	return clicks, nil
}
