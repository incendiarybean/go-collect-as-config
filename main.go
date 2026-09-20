package main

import (
	"fmt"
	"io/fs"
	"iter"
	"log"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/go-yaml/yaml"
	"golang.org/x/net/html"
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
	Type  string
	Attr  string
	Value string
	Child *ChildNode
}

type Article struct {
	Title     string
	Img       string
	Href      string
	Timestamp string
}

var UserAgents = []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64)", "AppleWebKit/537.36 (KHTML, like Gecko)", "Chrome/153.0.0.0", "Safari/537.36", "Edg/153.0.0.0"}

func RandomUserAgent() string {
	index := rand.IntN(len(UserAgents)-0) + 0

	return UserAgents[index]
}

type Callback func(*html.Node, string)

func getNodes(node *html.Node, child *ChildNode, callback Callback) {

	foundEntry := false
	var value string

	for _, attr := range node.Attr {
		if attr.Key == child.Attr && attr.Val == child.Value {
			foundEntry = true
		}

		if child.Type == node.Data {
			if child.Attr == attr.Key {
				value = attr.Val
				foundEntry = true
			}

			// if child.Attr != "" {
			// 	value = attr.Val
			// 	foundEntry = true
			// }
			// slog.Info(fmt.Sprintf("article: %s", article))
		}

		if attr.Key == child.Type && foundEntry == false {

			if attr.Key == "class" {
				classes := strings.Split(attr.Val, " ")
				if slices.Contains(classes, child.Value) {
					foundEntry = true
				}
			}

			if attr.Key == "id" && attr.Val == child.Value {
				foundEntry = true
			}

		}

	}

	if foundEntry == true {
		if child.Child != nil {
			for nextChild := range node.Descendants() {
				getNodes(nextChild, child.Child, callback)
			}
		} else {
			callback(node, value)
		}
	}
}

func getArticleData(articles []Article, rawArticle iter.Seq[*html.Node], config CollectorData) []Article {
	var article Article
	for node := range rawArticle {
		if config.Title != nil {
			getNodes(node, config.Title, func(node *html.Node, value string) {
				if node.FirstChild != nil {
					article.Title = node.FirstChild.Data
				} else {
					article.Title = node.Data
				}
			})
		}

		if config.Href != nil {
			getNodes(node, config.Href, func(node *html.Node, value string) {
				article.Href = value

			})
		}

		if config.Timestamp != nil {
			getNodes(node, config.Timestamp, func(node *html.Node, value string) {
				article.Timestamp = value
			})
		}

		if config.Img != nil {
			getNodes(node, config.Img, func(node *html.Node, value string) {
				article.Img = value
			})
		}
	}
	if article.Title != "" {
		slog.Info(fmt.Sprintf("article: %s", article))
	}

	return append(articles, article)
}

func collector(config Collector) {

	request, err := http.NewRequest("GET", config.Url, nil)
	if err != nil {
		slog.Error(fmt.Sprintf("Error creating request: %s", err.Error()))
	}

	request.Header.Set("User-Agent", RandomUserAgent())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		slog.Error(fmt.Sprintf("Error making request: %s", err.Error()))
	}
	defer response.Body.Close()

	slog.Info(fmt.Sprintf("%s - Response Status: %v", config.Name, response.StatusCode))

	// Parse document
	doc, err := html.Parse(response.Body)
	if err != nil {
		slog.Error(fmt.Sprintf("Error: %s", err.Error()))
	}

	// Use the YAML filters to get the articles
	var rawArticles []iter.Seq[*html.Node]
	for node := range doc.Descendants() {
		getNodes(node, config.Container, func(node *html.Node, value string) {
			rawArticles = append(rawArticles, node.Descendants())
		})
	}

	slog.Info(fmt.Sprintf("article: %v", len(rawArticles)))

	// Parse the articles into values
	var articles []Article
	for _, node := range rawArticles {

		// var b bytes.Buffer
		// html.Render(&b, node)
		// slog.Info(fmt.Sprintf("article: %v", b.String()))

		// slog.Info(fmt.Sprintf("article: %v", i))

		articles = getArticleData(articles, node, config.Data)
	}
	slog.Info(fmt.Sprintf("collected %v articles...", len(articles)))

}

func main() {
	configDir := os.DirFS("./config")
	collectorsConfig, err := fs.ReadFile(configDir, "collectors.yml")
	if err != nil {
		log.Fatal("Could not obtain collectors file, have they been provided?")
		os.Exit(1)
	}

	var collectorConfig CollectorConfig
	yaml.Unmarshal(collectorsConfig, &collectorConfig)

	for _, config := range collectorConfig.Collectors {
		collector(config)
	}
}
