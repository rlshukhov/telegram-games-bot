package main

import (
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
	"log"
	"net/http"
	"os"
	"strconv"
	"telegram-games-bot/games/clicks"
	"telegram-games-bot/games/reaction"
	"time"
)

var botToken = ""
var jwtSecret = ""
var listen = ""
var origin = ""

type Game interface {
	ShortName() string
	FS() *embed.FS
	ScoreCalculator(r *http.Request) (int, error)
}

var games = map[string]Game{
	reaction.Game.ShortName(): &reaction.Game,
	clicks.Game.ShortName():   &clicks.Game,
}

type claims struct {
	jwt.RegisteredClaims

	ChatID    string `json:"chat_id"`
	UserID    int64  `json:"user_id"`
	MessageID string `json:"message_id"`
	Game      string `json:"game"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	botToken = os.Getenv("BOT_TOKEN")
	jwtSecret = os.Getenv("JWT_SECRET")
	listen = os.Getenv("LISTEN")
	origin = os.Getenv("ORIGIN")

	pref := tele.Settings{
		Token:  botToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	go b.Start()

	b.Handle(tele.OnCallback, func(c tele.Context) error {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
			ChatID:    c.Callback().ChatInstance,
			UserID:    c.Callback().Sender.ID,
			MessageID: c.Callback().MessageID,
			Game:      c.Callback().GameShortName,
		})
		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			return err
		}

		return c.Respond(&tele.CallbackResponse{
			URL: origin + "/" + c.Callback().GameShortName + "?token=" + tokenString,
		})
	})

	b.Handle(tele.OnQuery, func(c tele.Context) error {
		results := make(tele.Results, len(games))
		var i int
		for _, game := range games {
			result := &tele.GameResult{
				ShortName: game.ShortName(),
			}

			results[i] = result
			results[i].SetResultID(strconv.Itoa(i))

			i = i + 1
		}

		return c.Answer(&tele.QueryResponse{
			Results:   results,
			CacheTime: 60,
		})
	})

	for _, g := range games {
		http.HandleFunc("/"+string(g.ShortName()), func(w http.ResponseWriter, r *http.Request) {
			html, _ := g.FS().ReadFile("index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, err := w.Write(html)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		})
	}

	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		jwtClaims, err := getClaims(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		scores, err := b.GameScores(&tele.User{ID: jwtClaims.UserID}, &tele.StoredMessage{
			MessageID: jwtClaims.MessageID,
		})
		if err != nil {
			http.Error(w, "internal scores error", http.StatusInternalServerError)
			return
		}

		for _, score := range scores {
			_, err := b.SetGameScore(score.User, &tele.StoredMessage{
				MessageID: jwtClaims.MessageID,
			}, tele.GameHighScore{
				Score: 0,
				Force: true,
			})
			if err != nil && !errors.Is(err, tele.ErrTrueResult) {
				http.Error(w, "internal scores error", http.StatusInternalServerError)
				return
			}

			time.Sleep(100 * time.Millisecond)
		}

		_, err = w.Write([]byte("Result reset!"))
		if err != nil {
			http.Error(w, "internal write error", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		jwtClaims, err := getClaims(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var currentGame Game
		if g, ok := games[jwtClaims.Game]; ok {
			currentGame = g
		} else {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		score, err := currentGame.ScoreCalculator(r)
		if err != nil {
			http.Error(w, "unexpected score", http.StatusBadRequest)
			return
		}

		_, err = b.SetGameScore(&tele.User{ID: jwtClaims.UserID}, &tele.StoredMessage{MessageID: jwtClaims.MessageID}, tele.GameHighScore{
			User:  &tele.User{ID: jwtClaims.UserID},
			Score: score,
		})
		if err != nil && !errors.Is(err, tele.ErrTrueResult) {
			log.Default().Println(err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		_, err = w.Write([]byte("Result received!"))
		if err != nil {
			http.Error(w, "internal write error", http.StatusInternalServerError)
			return
		}
	})

	fmt.Println("Server running on " + listen)
	err = http.ListenAndServe(listen, nil)
	if err != nil {
		log.Fatal(err)
		return
	}
}

func getClaims(r *http.Request) (claims, error) {
	if r.Method != http.MethodPost {
		return claims{}, errors.New("method not allowed")
	}

	err := r.ParseForm()
	if err != nil {
		return claims{}, err
	}

	tokenString := r.FormValue("token")
	token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(jwtSecret), nil
	})
	if err != nil {
		return claims{}, err
	}
	var jwtClaims claims
	if c, ok := token.Claims.(*claims); ok {
		jwtClaims = *c
	} else {
		return claims{}, errors.New("unauthorized")
	}

	return jwtClaims, nil
}
