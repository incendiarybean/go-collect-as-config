package collectors

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

type ChildNode struct {
	Type   string
	Attr   string
	Value  string
	Child  *ChildNode
	Prefix string
}

// A function type describing the callback from the getNodes function
type Callback func(*html.Node, string)

// A function to return the element nodes associated with the provided filters
// A callback is provided to return a given node, or value
func getNodes(node *html.Node, child *ChildNode, callback Callback) {
	foundEntry := false
	var value string

	if node != nil && child != nil {

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
			if attr.Key == child.Type && !foundEntry {
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
	}

	// If the entry was found in the DOM
	if foundEntry {
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
