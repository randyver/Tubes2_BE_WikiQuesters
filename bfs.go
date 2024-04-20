package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gammazero/deque"
)

type Solution []string

var wikiLinkRegex = regexp.MustCompile(`^/wiki/.*`)

func titleToUrl(title string) string {
	return ("https://en.wikipedia.org/wiki/" + strings.Join(strings.Split(title, " "), "_"))
}

func getHyperlinks(url string, visited *map[string]bool) map[string]bool {

	result := make(map[string]bool, 500)

	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Find the review items
	doc.Find("#content a").Each(func(i int, s *goquery.Selection) {

		// Get the hyperlink URL
		href, exist := s.Attr("href")

		if !exist {
			return // Skip if there's no href attribute
		}

		if !wikiLinkRegex.MatchString(href) {
			return
		}

		if (*visited)["https://en.wikipedia.org"+href] {
			return
		}

		// Get the hyperlink text (optional)
		result["https://en.wikipedia.org"+href] = true
		(*visited)["https://en.wikipedia.org"+href] = true
	})

	return result
}

func bfs(title1 string, title2 string) Solution {

	// Convert titles to URLs
	start := titleToUrl(title1)
	end := titleToUrl(title2)

	// Create a queue for storing solutions (paths)
	var theQueue deque.Deque[Solution]
	theQueue.PushFront([]string{start})

	// Visited URLs to avoid cycles
	visited := map[string]bool{start: true}

	// Loop until queue is empty or end is found
	for !(theQueue.Len() == 0) {
		// Dequeue the current path
		currentPath := theQueue.PopFront()
		fmt.Printf("Current Link : %s\n", currentPath[len(currentPath)-1])

		// Get hyperlinks from the last URL in the path
		hyperlinks := getHyperlinks(currentPath[len(currentPath)-1], &visited)

		if hyperlinks[end] {
			return append(currentPath, end)
		}

		// Iterate through hyperlinks
		for hyperlink := range hyperlinks {
			// Create a new solution by appending the hyperlink to the current path
			newPath := append(currentPath, hyperlink)
			// Push the new solution (extended path) to the back of the queue
			theQueue.PushBack(newPath)
		}
	}

	// Queue is empty and end not found, return an empty solution
	return []string{}
}

func main() {
	start := time.Now()
	result := bfs("Usain Bolt", "Slit lamp")
	execution_time := time.Since(start)

	for _, link := range result {
		fmt.Printf("%s\n", link)
	}

	fmt.Printf("execution time : %f seconds", execution_time.Seconds())
}
