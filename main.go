package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/line/line-bot-sdk-go/v7/linebot"
)

func main() {
	var port = os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	secret := os.Getenv("CHANNEL_SECRET")
	if secret == "" {
		log.Fatal("CHANNEL_SECRET must be set")
	}

	token := os.Getenv("CHANNEL_TOKEN")
	if token == "" {
		log.Fatal("CHANNEL_TOKEN must be set")
	}

	bot, err := linebot.New(
		secret,
		token,
	)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s\n", r.RequestURI)
		fmt.Fprintln(w, "Hello, World!")
	})

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s\n", r.RequestURI)
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=7776000")
		fmt.Fprintln(w, "data:image/x-icon;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQEAYAAABPYyMiAAAABmJLR0T///////8JWPfcAAAACXBIWXMAAABIAAAASABGyWs+AAAAF0lEQVRIx2NgGAWjYBSMglEwCkbBSAcACBAAAeaR9cIAAAAASUVORK5CYII=")
	})

	// Setup HTTP Server for receiving requests from LINE platform
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		events, err := bot.ParseRequest(r)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			return
		}
		for _, event := range events {
			log.Print("UserID: ", event.Source.UserID)
			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {
				case *linebot.TextMessage:
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(message.Text)).Do(); err != nil {
						log.Print(err)
					}
				case *linebot.StickerMessage:
					replyMessage := fmt.Sprintf(
						"sticker id is %s, stickerResourceType is %s", message.StickerID, message.StickerResourceType)
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(replyMessage)).Do(); err != nil {
						log.Print(err)
					}
				}
			}
		}
	})

	http.HandleFunc("/detect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		} else {
			w.WriteHeader(400)
		}

		if _, err := bot.PushMessage("U3c5ce2f10e91593849110b750e6a0d6b", linebot.NewImageMessage("https://pjreddie.com/media/image/Screen_Shot_2018-03-24_at_10.48.42_PM.png", "https://pjreddie.com/media/image/Screen_Shot_2018-03-24_at_10.48.42_PM.png")).Do(); err != nil {
			log.Print(err)
		}
	})
	// This is just sample code.
	// For actual use, you must support HTTPS by using `ListenAndServeTLS`, a reverse proxy or something else.
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
