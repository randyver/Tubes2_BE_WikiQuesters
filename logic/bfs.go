package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gammazero/deque"
)

type Solution []string

var queueLock sync.Mutex
var mapLock sync.Mutex
var resultLock sync.Mutex

var wikiLinkRegex = regexp.MustCompile(`^/wiki/.*`)
var bugRegex = regexp.MustCompile(`.*2024/.*`)

var reqwg sync.WaitGroup

var threadCount int = 200
var waitTime float64 = 15.0

func titleToUrl(title string) string {
	return ("https://en.wikipedia.org/wiki/" + strings.Join(strings.Split(title, " "), "_"))
}

func getUntil(p, ms string) string {
	i := strings.Index(ms, p)
	if i == -1 {
		return ""
	}
	return ms[0:i]
}

func getHyperlinks(url string, visited *map[string]bool) map[string]bool {

	result := make(map[string]bool)

	reqwg.Wait()
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		fmt.Printf("Failed while opening url : %s\n", url)
		fmt.Printf("status code error: %d %s\n", res.StatusCode, res.Status)

		if res.StatusCode == 429 {
			reqwg.Add(1)
			duration := time.Duration(waitTime * float64(time.Second))
			fmt.Printf("Waiting %d seconds. . . . .\n", int(duration.Seconds()))
			time.Sleep(duration)
			fmt.Printf("Continuing...\n")
			reqwg.Done()
			return getHyperlinks(url, visited)
		}
		fmt.Printf("\n")
		return result
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

		if bugRegex.MatchString(href) {
			href = getUntil("2024/", href)
		}

		mapLock.Lock()
		if (*visited)["https://en.wikipedia.org"+href] {
			mapLock.Unlock()
			return
		}

		// Get the hyperlink text (optional)
		result["https://en.wikipedia.org"+href] = true
		(*visited)["https://en.wikipedia.org"+href] = true
		mapLock.Unlock()
	})

	return result
}

func bfsMultiThread(title1 string, title2 string) ([]Solution, int, int) {

	var result []Solution

	// Convert titles to URLs
	start := titleToUrl(title1)
	end := titleToUrl(title2)

	QueriedPage := 0

	// Create a queue for storing solutions (paths)
	var theQueue deque.Deque[Solution]
	theQueue.PushFront([]string{start})

	// Visited URLs to avoid cycles
	visited := map[string]bool{start: true}

	var currentDepth int
	var wg sync.WaitGroup
	solFound := false
	var solLength int = 0

	for theQueue.Len() != 0 && (!solFound || (solFound && len(theQueue.Front()) == solLength-1)) {

		start := time.Now()
		currentDepth = len(theQueue.Front())
		for i := 0; i < threadCount; i++ {
			wg.Add(1)
			queueLock.Lock()
			if theQueue.Len() == 0 || len(theQueue.Front()) != currentDepth {
				wg.Done()
				queueLock.Unlock()
				break
			}
			go func() {
				if theQueue.Back()[len(theQueue.Back())-1] == end {
					solFound = true
					solLength = currentDepth + 1
					mapLock.Lock()
					visited[end] = false
					mapLock.Unlock()
					resultLock.Lock()
					result = append(result, theQueue.Back())
					resultLock.Unlock()
				}

				var currentLink string
				var currentHyperlinks map[string]bool

				currentPath := theQueue.PopFront()
				queueLock.Unlock()

				currentLink = currentPath[len(currentPath)-1]
				currentHyperlinks = getHyperlinks(currentLink, &visited)
				QueriedPage += 1

				if currentHyperlinks[end] {
					solFound = true
					solLength = currentDepth + 1
					mapLock.Lock()
					visited[end] = false
					mapLock.Unlock()
					resultLock.Lock()
					result = []Solution(append(result, append(currentPath, end)))
					resultLock.Unlock()
				} else {
					for iter := range currentHyperlinks {
						if !solFound {
							queueLock.Lock()
							var newItem Solution
							newItem = append(newItem, currentPath...)
							newItem = append(newItem, iter)

							theQueue.PushBack(newItem)
							queueLock.Unlock()
						}
					}
				}
				wg.Done()
			}()
		}
		wg.Wait()
		end := time.Since(start)
		fmt.Printf("[Got %d links in %d ms, average time per link : %f ms, made %f requests/second]\n", threadCount, end.Milliseconds(), float64(end.Milliseconds())/float64(threadCount), 1000*float64(threadCount)/float64(end.Milliseconds()))
	}

	return result, QueriedPage, len(visited)
}

func main() {
	start := time.Now()

	result, QueriedPageCount, ObtainedLinkCount := bfsMultiThread("Car", "Main_Page")

	// result := bfsMultiThread("Escalator etiquette", "Renier of Montferrat") : 527511 ms = 527.511 s = nyaris 14 menit
	// result := bfsMultiThread("3,4-Epoxycyclohexylmethyl-3',4'-epoxycyclohexane carboxylate", "Umbraculum umbraculum") : 5 separation

	execution_time := time.Since(start)

	for _, link := range result {
		fmt.Printf("%s\n", link)
	}
	fmt.Printf("Queried %d https pages\n", QueriedPageCount)
	fmt.Printf("Obtained %d links\n", ObtainedLinkCount)
	fmt.Printf("execution time : %d ms\n", execution_time.Milliseconds())
}
