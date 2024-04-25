package bfs

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

type QueueItem struct {
	name  string
	depth int
}

var queueLock sync.Mutex
var mapLock sync.Mutex
var graphLock sync.Mutex

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

func getHyperlinks(url string) map[string]bool {

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
			return getHyperlinks(url)
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

		// Get the hyperlink text (optional)
		result["https://en.wikipedia.org"+href] = true
	})

	return result
}

func existInGraph(name string, graph *map[string][]QueueItem) bool {
	_, exists := (*graph)[name]
	return exists
}

func BfsMultiThread(title1 string, title2 string) (map[string][]string, int64, int64, int) {

	startTime := time.Now()
	graph := make(map[string][]QueueItem)
	visited := make(map[string]bool)

	// Convert titles to URLs
	start := titleToUrl(title1)
	end := titleToUrl(title2)

	QueriedPage := int64(0)

	// Create a queue for storing solutions (paths)
	var theQueue deque.Deque[QueueItem]
	theQueue.PushFront(QueueItem{name: start, depth: 1})

	var currentDepth int
	var wg sync.WaitGroup
	solFound := false
	var solLength int = 0

	for theQueue.Len() != 0 && (!solFound || (solFound && theQueue.Front().depth == solLength-1)) {

		currentDepth = theQueue.Front().depth
		fmt.Printf("Currently at depth : %d\n", currentDepth)
		for i := 0; i < threadCount; i++ {
			wg.Add(1)
			queueLock.Lock()
			if theQueue.Len() == 0 || theQueue.Front().depth != currentDepth {
				wg.Done()
				queueLock.Unlock()
				break
			}
			go func() {
				if theQueue.Front().name == end {
					solFound = true
					solLength = currentDepth
				}

				var currentHyperlinks map[string]bool

				currentItem := theQueue.PopFront()
				currentLink := currentItem.name
				queueLock.Unlock()
				mapLock.Lock()
				if !visited[currentLink] {
					mapLock.Unlock()
					currentHyperlinks = getHyperlinks(currentLink)
					mapLock.Lock()
					visited[currentLink] = true
				}
				mapLock.Unlock()
				QueriedPage += 1

				if currentHyperlinks[end] {
					solFound = true
					solLength = currentDepth + 1
					graph[end] = append(graph[end], QueueItem{name: currentLink, depth: currentDepth})
				} else {
					for iter := range currentHyperlinks {
						graphLock.Lock()
						if existInGraph(iter, &graph) {
							if currentDepth == graph[iter][0].depth {
								graph[iter] = append(graph[iter], QueueItem{name: currentLink, depth: currentDepth})
							}
						} else {
							graph[iter] = []QueueItem{{name: currentLink, depth: currentDepth}}
						}
						graphLock.Unlock()
						mapLock.Lock()
						if !visited[iter] {
							if iter != end {
								queueLock.Lock()
								var newItem QueueItem
								newItem.name = iter
								newItem.depth = currentDepth + 1
								theQueue.PushBack(newItem)
								queueLock.Unlock()
							}
						}
						mapLock.Unlock()
					}
				}
				wg.Done()
			}()
		}
		wg.Wait()
	}
	elapsedTime := time.Since(startTime).Milliseconds()

	resultGraph := make(map[string][]string)
	var outputQueue deque.Deque[string]
	outputQueue.PushBack(end)
	for outputQueue.Len() != 0 {
		for _, item := range graph[outputQueue.Front()] {
			resultGraph[outputQueue.Front()] = append(resultGraph[outputQueue.Front()], item.name)
			if item.name != start {
				outputQueue.PushBack(item.name)
			}
		}
		outputQueue.PopFront()
	}

	return resultGraph, (elapsedTime), QueriedPage, int(solLength)
}

// func main() {
// 	result, time, visited, path_length := BfsMultiThread("Ostrich", "Camel")

// 	fmt.Println(result)
// 	fmt.Printf("Elapsed Time : %d ms, visited nodes : %d, path length : %d\n", time, visited, path_length)
// }
