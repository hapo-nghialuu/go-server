package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	channelSecret := os.Getenv("LINE_CHANNEL_SECRET")
	channelToken := os.Getenv("LINE_CHANNEL_TOKEN")

	bot, err := messaging_api.NewMessagingApiAPI(channelToken)
	if err != nil {
		log.Fatal(err)
	}

	// Setup HTTP Server for receiving requests from LINE platform
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			fmt.Fprint(w, "hello world")
			return
		}
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {
		log.Println("/callback called...")

		cb, err := webhook.ParseRequest(channelSecret, req)
		if err != nil {
			log.Printf("Cannot parse request: %+v\n", err)
			if errors.Is(err, webhook.ErrInvalidSignature) {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			return
		}

		log.Println("Handling events...")
		for _, event := range cb.Events {
			log.Printf("/ callback called %+v...\n", event)

			switch e := event.(type) {
			case webhook.MessageEvent:
				userId, errExtract := extractUserId(e.Source)
				log.Printf("/ Error Extract %+v...\n", errExtract)
				switch message := e.Message.(type) {
				case webhook.TextMessageContent:
					if message.Text == "連携する" {
						log.Printf("Initiating link token process.")

						// Phát hành linkToken
						linkToken, err := getLinkToken(channelToken, userId)
						if err != nil {
							log.Print(err)
							return
						}
						log.Printf("linkToken. %+v \n", linkToken)
						feLoginUrl := os.Getenv("FRONT_END_LOGIN_URL")
						loginURL := feLoginUrl + "?linkToken=" + linkToken
						if _, err := bot.ReplyMessage(
							&messaging_api.ReplyMessageRequest{
								ReplyToken: e.ReplyToken,
								Messages: []messaging_api.MessageInterface{
									messaging_api.TextMessage{
										Text: loginURL,
									},
								},
							},
						); err != nil {
							log.Print(err)
						} else {
							log.Println("Sent linkToken reply.")
						}
					} else {
						replyMessage := "あなたは" + message.Text + "と言いました。"
						if _, err = bot.ReplyMessage(
							&messaging_api.ReplyMessageRequest{
								ReplyToken: e.ReplyToken,
								Messages: []messaging_api.MessageInterface{
									messaging_api.TextMessage{
										Text: replyMessage,
									},
								},
							},
						); err != nil {
							log.Print(err)
						} else {
							log.Println("Sent text reply.")
						}
					}
				default:
					log.Printf("Unsupported message content: %T\n", message)
				}
			default:
				log.Printf("Unsupported event type: %T\n", event)
			}
		}
	})

	// For actual use, you must support HTTPS by using `ListenAndServeTLS`, a reverse proxy or something else.
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	fmt.Println("http://localhost:" + port + "/")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// extractUserId lấy user ID từ nguồn của sự kiện
func extractUserId(source webhook.SourceInterface) (string, error) {
	switch src := source.(type) {
	case webhook.UserSource:
		return src.UserId, nil
	default:
		return "", fmt.Errorf("unsupported source type: %T", source)
	}
}

// getLinkToken fetches the link token for the given user ID using the LINE API
func getLinkToken(channelToken, userID string) (string, error) {
	url := fmt.Sprintf("https://api.line.me/v2/bot/user/%s/linkToken", userID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+channelToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get link token: %s", resp.Status)
	}

	var result struct {
		LinkToken string `json:"linkToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.LinkToken, nil
}
