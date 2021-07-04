package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"context"

	"cloud.google.com/go/firestore"
	cloud "cloud.google.com/go/storage"
	firebase "firebase.google.com/go"

	"github.com/bookjan/line-bot-go/models"
	"github.com/gorilla/mux"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"google.golang.org/api/option"
)

type App struct {
	Router       *mux.Router
	ctx          context.Context
	client       *firestore.Client
	storage      *cloud.Client
	linebotCient *linebot.Client
}

func main() {
	route := App{}
	route.Init()
	route.Run()
}

func (route *App) Init() {

	route.ctx = context.Background()

	sa := option.WithCredentialsFile("serviceAccountKey.json")

	var err error

	app, err := firebase.NewApp(route.ctx, nil, sa)
	if err != nil {
		log.Fatalln(err)
	}

	route.client, err = app.Firestore(route.ctx)
	if err != nil {
		log.Fatalln(err)
	}

	route.storage, err = cloud.NewClient(route.ctx, sa)
	if err != nil {
		log.Fatalln(err)
	}

	route.Router = mux.NewRouter()
	route.initializeRoutes()
	fmt.Println("Successfully connected at port : " + route.GetPort())

	secret := os.Getenv("CHANNEL_SECRET")
	if secret == "" {
		log.Fatal("CHANNEL_SECRET must be set")
	}

	token := os.Getenv("CHANNEL_TOKEN")
	if token == "" {
		log.Fatal("CHANNEL_TOKEN must be set")
	}

	route.linebotCient, err = linebot.New(
		secret,
		token,
	)
	if err != nil {
		log.Fatal(err)
	}
}

func (route *App) GetPort() string {
	var port = os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	return ":" + port
}

func (route *App) Run() {
	log.Fatal(http.ListenAndServe(route.GetPort(), route.Router))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func (route *App) initializeRoutes() {
	route.Router.HandleFunc("/", route.Home).Methods("GET")
	route.Router.HandleFunc("/upload/image", route.UploadImage).Methods("POST")
	route.Router.HandleFunc("/upload/image/base64", route.UploadImageBase64).Methods("POST")
	route.Router.HandleFunc("/callback", route.lineServiceCallback).Methods("POST")
	route.Router.HandleFunc("/detect/image", route.DetectImage).Methods("POST")
}

func (route *App) Home(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, "Hello World!")
}

func (route *App) UploadImage(w http.ResponseWriter, r *http.Request) {
	file, handler, err := r.FormFile("image")
	r.ParseMultipartForm(10 << 20)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	imagePath := handler.Filename

	bucket := "ct-backend-7776d.appspot.com"

	wc := route.storage.Bucket(bucket).Object(imagePath).NewWriter(route.ctx)
	_, err = io.Copy(wc, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return

	}
	if err := wc.Close(); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	err = CreateImageUrl(imagePath, bucket, route.ctx, route.client)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, "Create image success.")
}

type DetectedImage struct {
	Data string
	Name string
}

func (route *App) UploadImageBase64(w http.ResponseWriter, r *http.Request) {
	var detectedImage DetectedImage
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&detectedImage)

	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	imagePath := "test_" + time.Now().String() + ".png"
	bucket := "ct-backend-7776d.appspot.com"

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(detectedImage.Data))
	wc := route.storage.Bucket(bucket).Object(imagePath).NewWriter(route.ctx)
	_, err = io.Copy(wc, reader)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return

	}
	if err := wc.Close(); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	err = CreateImageUrl(imagePath, bucket, route.ctx, route.client)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, "Create image success.")
}

// Setup HTTP Server for receiving requests from LINE platform
func (route *App) lineServiceCallback(w http.ResponseWriter, r *http.Request) {
	events, err := route.linebotCient.ParseRequest(r)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	for _, event := range events {
		log.Print("UserID: ", event.Source.UserID)
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				if _, err = route.linebotCient.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(message.Text)).Do(); err != nil {
					log.Print(err)
				}
			case *linebot.StickerMessage:
				replyMessage := fmt.Sprintf(
					"sticker id is %s, stickerResourceType is %s", message.StickerID, message.StickerResourceType)
				if _, err = route.linebotCient.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(replyMessage)).Do(); err != nil {
					log.Print(err)
				}
			}
		}
	}
}

func (route *App) DetectImage(w http.ResponseWriter, r *http.Request) {
	if _, err := route.linebotCient.BroadcastMessage(linebot.NewImageMessage("https://pjreddie.com/media/image/Screen_Shot_2018-03-24_at_10.48.42_PM.png", "https://pjreddie.com/media/image/Screen_Shot_2018-03-24_at_10.48.42_PM.png")).Do(); err != nil {
		log.Print(err)
		respondWithError(w, http.StatusBadRequest, err.Error())
	}

	respondWithJSON(w, http.StatusOK, "OK")
}

func CreateImageUrl(imagePath string, bucket string, ctx context.Context, client *firestore.Client) error {
	imageStructure := models.ImageStructure{
		ImageName: imagePath,
		URL:       "https://storage.cloud.google.com/" + bucket + "/" + imagePath,
	}

	_, _, err := client.Collection("image").Add(ctx, imageStructure)
	if err != nil {
		return err
	}

	return nil
}
