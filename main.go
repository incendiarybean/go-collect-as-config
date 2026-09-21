package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/lmittmann/tint"
	"golang.org/x/net/html"
	_ "modernc.org/sqlite"
)

type CollectorConfig struct {
	Collectors []Collector
}

type Collector struct {
	Name      string
	Url       string
	Container *ChildNode
	Data      CollectorData
}

type CollectorData struct {
	Title     *ChildNode
	Href      *ChildNode
	Img       *ChildNode
	Timestamp *ChildNode
}

type ChildNode struct {
	Type   string
	Attr   string
	Value  string
	Child  *ChildNode
	Prefix string
}

type Article struct {
	Hash      string `json:"hash"`
	Outlet    string `json:"outlet"`
	Title     string `json:"title"`
	Img       string `json:"img,omitempty"`
	Href      string `json:"href,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

var UserAgents = []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "AppleWebKit/537.36 (KHTML, like Gecko)", "Chrome/153.0.0.0", "Safari/537.36", "Edg/153.0.0.0"}

// Obtain a random user agent from the list of available entries
func RandomUserAgent() string {
	index := rand.IntN(len(UserAgents)-0) + 0

	return UserAgents[index]
}

// A function type describing the callback from the getNodes function
type Callback func(*html.Node, string)

// A function to return the element nodes associated with the provided filters
// A callback is provided to return a given node, or value
func getNodes(node *html.Node, child *ChildNode, callback Callback) {

	foundEntry := false
	var value string

	// If the type of child e.g. "a" is the data of node, we've found a matching element
	if child.Type == node.Data {
		foundEntry = true
	}

	for _, attr := range node.Attr {

		// If filter is by attribute, check if the given attribute matches the current node's attributes and if the value matches
		if attr.Key == child.Attr && attr.Val == child.Value {
			foundEntry = true
		}

		// If the node type is the same as the filtered child type and the attribution matches
		if child.Type == node.Data {
			if child.Attr == attr.Key {
				value = attr.Val
				foundEntry = true
			}

			if node.Type == html.ElementNode {
				foundEntry = true
			}
		}

		// If the type of attribute matches the filtered type and hasn't yet been found
		if attr.Key == child.Type && foundEntry == false {
			// If the type of attribute is class, split and filter on an expected class value
			if attr.Key == "class" {
				classes := strings.Split(attr.Val, " ")
				if slices.Contains(classes, child.Value) {
					foundEntry = true
				}
			}

			// If the type of attribute is id, check if it matches the given value
			if attr.Key == "id" && attr.Val == child.Value {
				foundEntry = true
			}
		}

	}

	// If the entry was found in the DOM
	if foundEntry == true {
		// If the entry was found, but there are further filters to be applied - search deeper in the DOM
		if child.Child != nil {
			for nextChild := range node.Descendants() {
				getNodes(nextChild, child.Child, callback)
			}

		} else {
			// If the entry was found, but there are no further filters - return the current node and any assigned value
			callback(node, value)
		}
	}
}

func getArticleData(rawArticle *html.Node, config CollectorData, outlet string) Article {
	var article Article

	article.Outlet = outlet

	// For each available article found in the previous functions, render a new article
	for node := range rawArticle.Descendants() {

		// If the configuration has a Title search
		if config.Title != nil {
			getNodes(node, config.Title, func(node *html.Node, value string) {
				if article.Title == "" {
					var title string
					if node.FirstChild != nil {
						title = node.FirstChild.Data
					} else {
						title = node.Data
					}

					article.Title = strings.TrimSpace(title)
				}
			})
		}

		// If the configuration has a Href search
		if config.Href != nil {
			getNodes(node, config.Href, func(node *html.Node, value string) {
				if article.Href == "" {
					article.Href = config.Href.Prefix + value
				}
			})
		}

		// If the configuration has a Timestamp search
		if config.Timestamp != nil {
			getNodes(node, config.Timestamp, func(node *html.Node, value string) {
				if article.Timestamp == "" {
					article.Timestamp = value
				}
			})
		}

		// If the configuration has an Img search
		if config.Img != nil {
			getNodes(node, config.Img, func(node *html.Node, value string) {
				if article.Img == "" {
					img := config.Img.Prefix + value
					url := strings.Split(img, "?")
					article.Img = html.UnescapeString(url[0])
				}
			})
		}
	}

	// Generate the ID associated with the article's content
	article.Hash = hash(fmt.Sprintf("%s", article))

	return article
}

// A function to generate a sha1 hash, for monitoring duplications within the data store
func hash(str string) string {
	hasher := sha1.New()
	hasher.Write([]byte(str))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

// A function to make a request to the configured location and obtain fresh HTML content
func collector(channel chan Article, config Collector) {
	request, err := http.NewRequest("GET", config.Url, nil)
	if err != nil {
		slog.Error(fmt.Sprintf("Error creating request: %s", err.Error()))
		return
	}

	// Set a user-agent as systems have began blocking automated requests
	request.Header.Set("User-Agent", RandomUserAgent())

	slog.Debug(fmt.Sprintf("Making request for: %s", config.Url))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		slog.Error(fmt.Sprintf("Error making request: %s", err.Error()))
		return
	}
	slog.Debug(fmt.Sprintf("%s - Response Status: %v", config.Name, response.StatusCode))
	defer response.Body.Close()

	// Parse the docuemnt
	doc, err := html.Parse(response.Body)
	if err != nil {
		slog.Error(fmt.Sprintf("Error: %s", err.Error()))
		return
	}

	// Use the YAML filters to get the articles
	var rawArticles []*html.Node
	for node := range doc.Descendants() {
		getNodes(node, config.Container, func(node *html.Node, value string) {
			rawArticles = append(rawArticles, node)
		})
	}

	// Parse the raw html nodes into articles
	var articles []Article
	for _, node := range rawArticles {
		article := getArticleData(node, config.Data, config.Name)
		articles = append(articles, article)

		// Send the article via channel to be stored
		channel <- article
	}

	slog.Info(fmt.Sprintf("Collected %v articles...", len(articles)))

}

// A function to concurrently run all collectors
func RunCollectors(channel chan Article, collectorConfig CollectorConfig, db *sql.DB) {
	for _, config := range collectorConfig.Collectors {
		slog.Info(fmt.Sprintf("Handling configuration for: %s", config.Name))
		go collector(channel, config)
	}
}

// A function to monitor for new articles via channel messages
func HandleCollectorMessages(channel chan Article, db *sql.DB) {
	statement, err := db.Prepare("INSERT INTO articles (Hash, title, href, img, timestamp, outlet) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		slog.Error(fmt.Sprintf("Error generating prepared statement: %s", err))
		return
	}

	for article := range channel {
		statement.Query(article.Hash, article.Title, article.Href, article.Img, article.Timestamp, article.Outlet)
	}
}

func main() {
	newsChannel := make(chan Article)

	// Create a new coloured logger
	slog.SetDefault(slog.New(
		tint.NewTextHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.RFC3339,
		}),
	))

	// Create or connect to a local storage
	db, err := sql.Open("sqlite", "./store.db")
	if err != nil {
		slog.Error(fmt.Sprintf("Could not obtain DB. %s", err.Error()))
		os.Exit(1)
	}

	// Create the SQLite table
	db.Exec("CREATE TABLE IF NOT EXISTS articles (hash TEXT primary key, title TEXT, href TEXT, img TEXT, timestamp TEXT, outlet TEXT)")

	// Read the local configuration file
	configDir := os.DirFS("./config")
	collectorsConfig, err := fs.ReadFile(configDir, "collectors.yml")
	if err != nil {
		slog.Error("Could not obtain collectors file, have they been provided?")
		os.Exit(1)
	}

	// Parse the YAML configuration
	var collectorConfig CollectorConfig
	yaml.Unmarshal(collectorsConfig, &collectorConfig)

	// Create a message queue and run the collectors
	go HandleCollectorMessages(newsChannel, db)
	go RunCollectors(newsChannel, collectorConfig, db)

	router := http.NewServeMux()

	// Generic routing to display data
	router.Handle("GET /", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		rows, err := db.Query("SELECT * FROM articles;")
		if err != nil {
			slog.Error("Could not query table.")
			http.Error(response, http.StatusText(502), 502)
			return
		}
		defer rows.Close()

		var articles []Article

		for rows.Next() {
			var article Article

			if err := rows.Scan(&article.Hash, &article.Title, &article.Href, &article.Img, &article.Timestamp, &article.Outlet); err != nil {
				slog.Error(err.Error())
				http.Error(response, http.StatusText(502), 502)
				return
			}
			articles = append(articles, article)
		}

		if err = rows.Err(); err != nil {
			slog.Error(err.Error())
			http.Error(response, http.StatusText(502), 502)
			return
		}

		response.Header().Set("content-type", "application/json")

		json.NewEncoder(response).Encode(articles)
	}))

	http.ListenAndServe(":8080", router)
}
