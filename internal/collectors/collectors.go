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

	"github.com/go-yaml/yaml"
	"github.com/incendiarybean/benweare.co.uk-collector.go/internal/utils"
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

type ApiResult struct {
	Type  *string
	Index *int8
}

type Worker struct {
	Name      string
	Style     string
	Url       string
	Params    map[string]string
	Result    *ApiResult
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

	switch self.Style {
	case "web":
		{
			self.webHandler(response.Body, outlet, channel)
			return
		}
	case "api":
		{
			self.apiHandler(response.Body, outlet, channel)
			return
		}
	}
}

func (self *Worker) webHandler(body io.ReadCloser, outlet string, channel *chan Article) {
	var articles []Article

	// Parse the docuemnt
	doc, err := html.Parse(body)
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

func (self *Worker) apiHandler(body io.ReadCloser, outlet string, channel *chan Article) {
	bodyBytes, _ := io.ReadAll(body)

	var content map[string]any
	if self.Result != nil {
		if self.Result.Type != nil && *self.Result.Type == "array" {
			var index = 0
			if self.Result.Index != nil {
				index = int(*self.Result.Index)
			}

			var contents []map[string]any
			if err := json.Unmarshal(bodyBytes, &contents); err != nil {
				slog.Error(err.Error())
				return
			}

			content = contents[index]
		} else {
			if err := json.Unmarshal(bodyBytes, &content); err != nil {
				slog.Error(err.Error())
				return
			}
		}
	}

	data := self.Data
	article := NewArticle(&self.Data, outlet)
	article.SetOutlet(outlet)
	article.SetTitle(nil, content[data.Title.Attr].(string))
	article.SetHref(nil, content[data.Href.Attr].(string))
	article.SetImg(nil, content[data.Img.Attr].(string))
	article.SetTimestamp(nil, content[data.Timestamp.Attr].(string))
	article.SetDescription(nil, content[data.Description.Attr].(string))
	article.GenerateId()

	slog.Info("Collected external API article...")

	// Send the article via channel to be stored
	*channel <- article
}
