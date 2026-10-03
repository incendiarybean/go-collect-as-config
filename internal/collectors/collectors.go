package collectors

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"benweare.co.uk/collector/internal/utils"
	"github.com/go-yaml/yaml"
	"golang.org/x/net/html"
)

type CollectorConfig struct {
	Collectors []Worker
}

type Collector struct {
	channel *chan Article
	db      *sql.DB
	workers []Worker
}

type Worker struct {
	Name      string
	Style     string
	Url       string
	Params    map[string]string
	Headers   map[string]string
	Container *ChildNode
	Data      WorkerData
}

type WorkerData struct {
	Title       *ChildNode
	Href        *ChildNode
	Img         *ChildNode
	Timestamp   *ChildNode
	Description *ChildNode
}

func New(pathToDatastore string, pathToConfig string) Collector {
	channel := make(chan Article, 1)

	// Create or connect to a local storage
	db, err := sql.Open("sqlite", pathToDatastore)
	if err != nil {
		slog.Error(fmt.Sprintf("Could not obtain DB. %s", err.Error()))
		os.Exit(1)
	}

	// Create the SQLite table
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS articles (id TEXT primary key, hash TEXT, title TEXT, href TEXT, img TEXT, timestamp TEXT, outlet TEXT, description TEXT)")

	// Read the local configuration file
	configDir := os.DirFS(pathToConfig)
	rawCollectorsConfig, err := fs.ReadFile(configDir, "collectors.yml")
	if err != nil {
		slog.Error("Could not obtain collectors file, have they been provided?")
		os.Exit(1)
	}

	// Parse the YAML configuration
	var workerConfig CollectorConfig
	err = yaml.Unmarshal(rawCollectorsConfig, &workerConfig)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("%v", workerConfig))

	return Collector{channel: &channel, workers: workerConfig.Collectors, db: db}
}

// A function to concurrently run all collectors
func (self Collector) Run() {
	slog.Info(fmt.Sprintf("%v", self.workers))
	for _, worker := range self.workers {
		slog.Info(fmt.Sprintf("Handling configuration for: %s", worker.Name))
		go worker.Create(self.channel)
	}

	go func() {
		statement, err := self.db.Prepare("INSERT INTO articles (id, Hash, title, href, img, timestamp, outlet, description) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			slog.Error(fmt.Sprintf("Error generating prepared statement: %s", err))
			return
		}

		for msg := range *self.channel {
			article := msg.Read()
			_, _ = statement.Query(article.Id, article.Hash, article.Title, article.Href, article.Img, article.Timestamp, article.Outlet, article.Description)
		}
	}()
}

// A function to monitor for new articles via channel messages
func (self *Collector) GetDB() *sql.DB {
	if self.db == nil {
		slog.Error("No DB has been intialised!")
		os.Exit(1)
	}
	return self.db
}

func (self *Worker) Create(channel *chan Article) {
	url := self.Url
	if self.Params != nil {
		url += "?"
		for key, value := range self.Params {
			if strings.Contains(value, "ENV_") {
				envValue := strings.Split(value, "ENV_")[1]
				value = os.Getenv(envValue)
				url += fmt.Sprintf("%s=%s", key, value)
			}
		}
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error(fmt.Sprintf("Error creating request: %s", err.Error()))
		return
	}

	if self.Headers != nil {
		for _, headerPair := range self.Headers {
			slog.Info(headerPair)
		}
	}

	// Set a user-agent as systems have began blocking automated requests
	request.Header.Set("User-Agent", utils.RandomUserAgent())

	slog.Debug(fmt.Sprintf("Making request for: %s", self.Url))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		slog.Error(fmt.Sprintf("Error making request: %s", err.Error()))
		return
	}
	slog.Debug(fmt.Sprintf("%s - Response Status: %v", self.Name, response.StatusCode))
	defer func() {
		_ = response.Body.Close()
	}()

	outlet := strings.ReplaceAll(self.Name, " ", "-")

	var articles []Article

	switch self.Style {
	case "web":
		{
			// Parse the docuemnt
			doc, err := html.Parse(response.Body)
			if err != nil {
				slog.Error(fmt.Sprintf("Error: %s", err.Error()))
				return
			}

			// Use the YAML filters to get the articles
			var rawArticles []*html.Node
			for node := range doc.Descendants() {
				getNodes(node, self.Container, func(node *html.Node, value string) {
					rawArticles = append(rawArticles, node)
				})
			}

			// Parse the raw html nodes into articles
			for _, rawArticle := range rawArticles {
				article := NewArticle(&self.Data, outlet).Write(rawArticle)
				articles = append(articles, article)

				// Send the article via channel to be stored
				*channel <- article
			}

			slog.Info(fmt.Sprintf("Collected %v articles...", len(articles)))
		}
	case "api":
		{
			bodyBytes, _ := io.ReadAll(response.Body)
			var content map[string]string
			_ = json.Unmarshal(bodyBytes, &content)

			slog.Info(fmt.Sprintf("%v", content))

			data := self.Data
			article := NewArticle(&self.Data, outlet)
			article.SetOutlet(outlet)
			article.SetTitle(nil, content[data.Title.Attr])
			article.SetHref(nil, content[data.Href.Attr])
			article.SetImg(nil, content[data.Img.Attr])
			article.SetTimestamp(nil, content[data.Timestamp.Attr])
			article.SetDescription(nil, content[data.Description.Attr])

			slog.Info("Collected external API article...")

			// Send the article via channel to be stored
			*channel <- article
		}
	}
}

// // A function to make a request to the configured location and obtain fresh HTML content
// func collector(channel chan Article, config Collector) {
// 	url := config.Url
// 	if config.Params != nil {
// 		url += "?"
// 		for key, value := range config.Params {
// 			if strings.Contains(value, "ENV_") {
// 				envValue := strings.Split(value, "ENV_")[1]
// 				value = os.Getenv(envValue)
// 				url += fmt.Sprintf("%s=%s", key, value)
// 			}
// 		}
// 	}

// 	request, err := http.NewRequest("GET", url, nil)
// 	if err != nil {
// 		slog.Error(fmt.Sprintf("Error creating request: %s", err.Error()))
// 		return
// 	}

// 	if config.Headers != nil {
// 		for _, headerPair := range config.Headers {
// 			slog.Info(headerPair)
// 		}
// 	}

// 	// Set a user-agent as systems have began blocking automated requests
// 	request.Header.Set("User-Agent", utils.RandomUserAgent())

// 	slog.Debug(fmt.Sprintf("Making request for: %s", config.Url))
// 	response, err := http.DefaultClient.Do(request)
// 	if err != nil {
// 		slog.Error(fmt.Sprintf("Error making request: %s", err.Error()))
// 		return
// 	}
// 	slog.Debug(fmt.Sprintf("%s - Response Status: %v", config.Name, response.StatusCode))
// 	defer func() {
// 		_ = response.Body.Close()
// 	}()

// 	outlet := strings.ReplaceAll(config.Name, " ", "-")

// 	var articles []Article

// 	switch config.Style {
// 	case "web":
// 		{
// 			// Parse the docuemnt
// 			doc, err := html.Parse(response.Body)
// 			if err != nil {
// 				slog.Error(fmt.Sprintf("Error: %s", err.Error()))
// 				return
// 			}

// 			// Use the YAML filters to get the articles
// 			var rawArticles []*html.Node
// 			for node := range doc.Descendants() {
// 				getNodes(node, config.Container, func(node *html.Node, value string) {
// 					rawArticles = append(rawArticles, node)
// 				})
// 			}

// 			// Parse the raw html nodes into articles
// 			for _, rawArticle := range rawArticles {
// 				article := NewArticle(&config.Data, outlet).Write(rawArticle)
// 				articles = append(articles, article)

// 				// Send the article via channel to be stored
// 				channel <- article
// 			}

// 			slog.Info(fmt.Sprintf("Collected %v articles...", len(articles)))
// 		}
// 	case "api":
// 		{
// 			bodyBytes, _ := io.ReadAll(response.Body)
// 			var content map[string]string
// 			_ = json.Unmarshal(bodyBytes, &content)

// 			article := NewArticle(&config.Data, outlet)
// 			article.SetOutlet(outlet)
// 			article.SetTitle(nil, content[config.Data.Title.Attr])
// 			article.SetHref(nil, content[config.Data.Href.Attr])
// 			article.SetImg(nil, content[config.Data.Img.Attr])
// 			article.SetTimestamp(nil, content[config.Data.Timestamp.Attr])
// 			article.SetDescription(nil, content[config.Data.Description.Attr])

// 			slog.Info("Collected external API article...")

// 			// Send the article via channel to be stored
// 			channel <- article
// 		}
// 	}

// }

// // A function to concurrently run all collectors
// func RunCollectors(channel chan Article, collectorConfig CollectorConfig, db *sql.DB) {
// 	for _, config := range collectorConfig.Collectors {
// 		slog.Info(fmt.Sprintf("Handling configuration for: %s", config.Name))
// 		go collector(channel, config)
// 	}
// }

// // A function to monitor for new articles via channel messages
// func HandleCollectorMessages(channel chan Article, db *sql.DB) {
// 	statement, err := db.Prepare("INSERT INTO articles (id, Hash, title, href, img, timestamp, outlet, description) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
// 	if err != nil {
// 		slog.Error(fmt.Sprintf("Error generating prepared statement: %s", err))
// 		return
// 	}

// 	for msg := range channel {
// 		article := msg.Read()
// 		_, _ = statement.Query(article.Id, article.Hash, article.Title, article.Href, article.Img, article.Timestamp, article.Outlet, article.Description)
// 	}
// }
