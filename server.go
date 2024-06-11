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
									&messaging_api.TemplateMessage{
										AltText: "Buttons template",
										Template: &messaging_api.ButtonsTemplate{
											Title: "アカウント連携開始",
											Text: "連携を開始します。リンク先でログイン\nを行なってください。",
											Actions: []messaging_api.ActionInterface{
												&messaging_api.UriAction{
													Label: "連携開始",
													Uri: loginURL,
												},
											},
										},
									},
								},
							},
						); err != nil {
							log.Print(err)
						} else {
							log.Println("Sent linkToken reply.")
						}
					} else {
						replyMessage := ""
						if message.Text == "連携解除" {
							err := requestUnlinkAccount(channelToken, userId)
							if err != nil {
								replyMessage = "連携解除が失敗しました。"
								log.Print(err)
							} else {
								replyMessage = "連携解除が完了しました。"
								log.Println("Unlink account successfully")
							}
						} else {
							replyMessage = "あなたは" + message.Text + "と言いました。"
						}
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
			case webhook.AccountLinkEvent:
				result := e.Link.Result
				log.Print(result)
				switch result {
					case "ok":
						replyMessage := "アカウント連携が完了しました。"
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
					case "failed":
						replyMessage := "アカウント連携が失敗しました。"
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
					default: {
						log.Printf("User verification fails")
					}
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

func requestUnlinkAccount(channelToken string, userID string) error {
	url := fmt.Sprintf("https://api.line.me/v2/bot/user/%s/richmenu", userID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+channelToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	log.Print(resp.Body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to unlink account: %s", resp.Status)
	}

	return nil
}
