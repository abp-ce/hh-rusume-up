package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"
)

const hhApiUrl = "https://api.hh.ru"

type EnvVars struct {
	clientId string
	clientSecret string
	telegramToken string
	telegramChatId string
	accessToken string
	refreshToken string
}

func MakeEnvVars(logger *log.Logger) *EnvVars {
	err := godotenv.Load(".env")

	if err != nil {
		logger.Fatal(err)
	}

	return &EnvVars{
		clientId: os.Getenv("CLIENT_ID"),
		clientSecret: os.Getenv("CLIENT_SECRET"),
		telegramToken: os.Getenv("TELEGRAM_TOKEN"),
		telegramChatId: os.Getenv("TELEGRAM_CHAT_ID"),
		accessToken: os.Getenv("ACCESS_TOKEN"),
		refreshToken: os.Getenv("REFRESH_TOKEN"),
	}
}

func SendToTelegram(message string, envVars *EnvVars, logger *log.Logger) {
	url := "https://api.telegram.org/bot" + envVars.telegramToken + "/sendMessage?chat_id=" + envVars.telegramChatId + "&text=" + message
	resp, err := http.Get(url)
	if err != nil {
		logger.Println("Error sending telegram message:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Println("Telegram status code error", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Println("Error reading telegram response:", err)
		return
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		logger.Println("Error decoding telegram JSON:", err)
		return
	}
}

func saveEnvVarsToFile(envVars *EnvVars, logger *log.Logger) {
	// Create a new file
	file, err := os.Create(".env")
	if err != nil {
		logger.Fatal(err)
	}
	defer file.Close()

	// Write the environment variables to the file
	file.WriteString("CLIENT_ID=" + envVars.clientId + "\n")
	file.WriteString("CLIENT_SECRET=" + envVars.clientSecret + "\n")
	file.WriteString("TELEGRAM_TOKEN=" + envVars.telegramToken + "\n")
	file.WriteString("TELEGRAM_CHAT_ID=" + envVars.telegramChatId + "\n")
	file.WriteString("ACCESS_TOKEN=" + envVars.accessToken + "\n")
	file.WriteString("REFRESH_TOKEN=" + envVars.refreshToken + "\n")

	logger.Println("Environment variables saved to .env file")
}

func refreshToken(envVars *EnvVars, logger *log.Logger) {
	// Отправляем запрос на обновление токена в форме x-www-form-urlencoded
    data := url.Values{}
    data.Set("grant_type", "refresh_token")
    data.Set("refresh_token", envVars.refreshToken)
	resp, err := http.PostForm(hhApiUrl + "/token", data)
	if err != nil {
		logger.Println("Error sending refresh request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Println("Refresh status code error", resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Println("Error reading refresh response:", err)
			return
		}
		logger.Println(string(body))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Println("Error reading refresh response:", err)
		return
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		logger.Println("Error decoding mine JSON:", err)
		return
	}

	envVars.accessToken = result["access_token"].(string)
	envVars.refreshToken = result["refresh_token"].(string)
	logger.Println("Access token refreshed")

	saveEnvVarsToFile(envVars, logger)
}

func printAllMine(result map[string]interface{}, logger *log.Logger) {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		logger.Println("Error coding mine JSON:", err)
		return
	}

	logger.Println(string(jsonData))
}

func getAllMine(envVars *EnvVars, logger *log.Logger) []byte {
	client := &http.Client{}
	req, err := http.NewRequest("GET", hhApiUrl + "/resumes/mine", nil)
	if err != nil {
		logger.Println("Error creating request:", err)
		return nil
	}

	req.Header.Set("Authorization", "Bearer " + envVars.accessToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		logger.Println("Error sending request:", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Println("Mine status code error", resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Println("Error reading mine response:", err)
			return nil
		}
		logger.Println(string(body))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Println("Error reading mine response:", err)
		return nil
	}

	return body
}

func resumeUpdate(resumeId string, envVars *EnvVars, logger *log.Logger) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", hhApiUrl + "/resumes/" + resumeId + "/publish", nil)
	if err != nil {
		logger.Println("Error creating up request:", err)
		return
	}

	req.Header.Set("Authorization", "Bearer " + envVars.accessToken)

	resp, err := client.Do(req)
	if err != nil {
		logger.Println("Error sending up request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		logger.Println("Resume update status code", resp.StatusCode)
		return
	}

	logger.Println("Resume updated")
}

func main() {
	// Для отладки
	printAll := flag.Bool("print", false, "Print all resumes")
	flag.Parse()

    logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
	if !*printAll {
		logger.SetOutput(&lumberjack.Logger{
			Filename:   "resume_up.log",
			MaxSize:    1,
			MaxBackups: 3,
			MaxAge:     30,
			Compress:   true,
		})
	}

	envVars := MakeEnvVars(logger)

	body := getAllMine(envVars, logger)
	if body == nil {
		refreshToken(envVars, logger)
		body = getAllMine(envVars, logger)
	}

	// Преобразуем JSON в map
	var result map[string]interface{}
	err := json.Unmarshal(body, &result)
	if err != nil {
		logger.Println("Error decoding mine JSON:", err)
		return
	}

	if *printAll {
		printAllMine(result, logger)
	}

	if items, ok := result["items"].([]interface{}); ok {
		for _, item := range items {
			if item, ok := item.(map[string]interface{}); ok {
				resumeId := item["id"].(string)
				resumeTitle := item["title"].(string)
				logger.Println(resumeTitle + " " + resumeId)
				// Проверка на видимость для клиентов
				if access, ok := item["access"].(map[string]interface{}); ok {
					if accessType, ok := access["type"].(map[string]interface{}); ok {
						if accessType["id"].(string) != "clients" {
							logger.Println("Not visible for clients")
							continue
						}
					}
				}
				if cp, ok := item["can_publish_or_update"].(bool); ok && cp {
					resumeUpdate(resumeId, envVars, logger)
					SendToTelegram("Успешно поднято " + resumeTitle, envVars, logger)
				} else {
					iso8601Format := "2006-01-02T15:04:05-0700"
					pubTime, err := time.Parse(iso8601Format, item["next_publish_at"].(string))
					if err != nil {
						logger.Println("Error parsing update time", err)
						return
					}
					logger.Printf("%s can publish or update at %s", item["title"].(string), pubTime.Format("02.01.2006 15:04"))
				}
			}
		}
	} else {
		logger.Println("Error: items not odject")
		return
	}
}