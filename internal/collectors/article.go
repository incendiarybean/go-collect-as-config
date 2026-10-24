package collectors

import (
	"fmt"
	"strings"
	"uuid"

	"github.com/incendiarybean/go-collect-as-config/internal/utils"
	"golang.org/x/net/html"
)

// Inner article content
type ArticleInner struct {
	Id          string `json:"id"`
	Hash        string `json:"hash"`
	Outlet      string `json:"outlet"`
	Title       string `json:"title"`
	Img         string `json:"img,omitempty"`
	Href        string `json:"href,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	Description string `json:"description,omitempty"`
}

// Outer article struct
type Article struct {
	Inner *ArticleInner
	Data  *WorkerData
}

// A function to create a new article
func NewArticle(config *WorkerData, outlet string) Article {
	return Article{
		Inner: &ArticleInner{Outlet: outlet},
		Data:  config,
	}
}

// A function to set the outlet
func (self *Article) SetOutlet(value string) {
	self.Inner.Outlet = value
}

// A function set the article title
func (self *Article) SetTitle(node *html.Node, value string) {
	if self.Data.Title != nil && self.Inner.Title == "" {
		title := value

		if node != nil {
			if node.FirstChild != nil {
				title = node.FirstChild.Data
			} else {
				title = node.Data
			}
		}

		self.Inner.Title = strings.TrimSpace(title)
	}
}

// A function set the article href
func (self *Article) SetHref(node *html.Node, value string) {
	if self.Data.Href != nil && self.Inner.Href == "" {
		self.Inner.Href = self.Data.Href.Prefix + value
	}
}

// A function set the article timestamp
func (self *Article) SetTimestamp(node *html.Node, value string) {
	if self.Data.Timestamp != nil && self.Inner.Timestamp == "" {
		self.Inner.Timestamp = value
	}
}

// A function set the article img
func (self *Article) SetImg(node *html.Node, value string) {
	if self.Data.Img != nil && self.Inner.Img == "" {
		img := self.Data.Img.Prefix + value
		url := strings.Split(img, "?")
		self.Inner.Img = html.UnescapeString(url[0])
	}
}

// A function set the article description
func (self *Article) SetDescription(node *html.Node, value string) {
	if self.Data.Description != nil && self.Inner.Description == "" {
		description := self.Data.Description.Prefix + value
		self.Inner.Description = description
	}
}

// Generate the ID associated with the article's content
func (self *Article) GenerateId() {
	self.Inner.Hash = utils.Hash(fmt.Sprintf("%s", self.Inner))
	self.Inner.Id = uuid.NewV7().String()
}

// A function write the article based on provided HTML nodes
func (self Article) Write(rawArticle *html.Node) Article {

	// For each available article found in the previous functions, render a new article
	for htmlNode := range rawArticle.Descendants() {
		// If the configuration has a Title search
		getNodes(htmlNode, self.Data.Title, self.SetTitle)

		// If the configuration has a Href search
		getNodes(htmlNode, self.Data.Href, self.SetHref)

		// If the configuration has a Timestamp search
		getNodes(htmlNode, self.Data.Timestamp, self.SetTimestamp)

		// If the configuration has an Img search
		getNodes(htmlNode, self.Data.Img, self.SetImg)
	}

	self.GenerateId()

	return self
}

// Read the inner article content
func (self Article) Read() ArticleInner {
	return *self.Inner
}
