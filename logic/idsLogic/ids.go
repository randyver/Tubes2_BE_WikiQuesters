package ids

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

type Solution map[string][]string
type SolutionDistance map[string]int

var visitedCount int64

var wg sync.WaitGroup
var reqWg sync.WaitGroup
var writeLock sync.Mutex
var readLock sync.Mutex
var threadLock sync.Mutex
var readCount uint64
var threadCount uint8

var wikiLinkRegex = regexp.MustCompile(`^/wiki/.*`)
var bugRegex = regexp.MustCompile(`.*2024/.*`)

func TitleToUrl(title string) string {
	return ("https://en.wikipedia.org/wiki/" + strings.Join(strings.Split(title, " "), "_"))
}

func getUntil(p, ms string) string {
	i := strings.Index(ms, p)
	if i == -1 {
		return ""
	}
	return ms[0:i]
}

func getHyperlinks(url string, nearbyNode *map[string][]string) {
	defer wg.Done()

	var result []string
	eksis := make(map[string]bool)

	reqWg.Wait()
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		fmt.Printf("Failed while opening url : %s\n", url)
		fmt.Printf("status code error: %d %s\n\n", res.StatusCode, res.Status)

		if res.StatusCode == 429 {
			reqWg.Add(1)
			time.Sleep(15 * time.Second)
			reqWg.Done()
			wg.Add(1)
			getHyperlinks(url, nearbyNode)
			return
		}
		return
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

		newNode := "https://en.wikipedia.org" + href
		if !eksis[newNode] && newNode != url {
			// fmt.Println(newNode)
			eksis[newNode] = true
			result = append(result, "https://en.wikipedia.org"+href)
		}
	})
	// fmt.Println(" source: ", url)
	writeLock.Lock()
	(*nearbyNode)[url] = result
	// fmt.Println(threadCount)
	writeLock.Unlock()
}

func dls(source string, target string, currentDepth int, maxDepth int, nearbyNode *map[string][]string, parent *map[string][]string, nodeVisited *map[string]bool, childVisited *map[string]bool, closestDist *map[string]int) {
	if currentDepth > 0 {
		(*nodeVisited)[source] = true
		// fmt.Println("curDepth: ", currentDepth, " source: ", source)
		if !(*childVisited)[source] {
			visitedCount += 1
			// read current nearby node
			readLock.Lock()
			readCount += 1
			if readCount == 1 {
				writeLock.Lock()
			}
			readLock.Unlock()
			currentNearby := (*nearbyNode)[source]
			readLock.Lock()
			readCount -= 1
			if readCount == 0 {
				writeLock.Unlock()
			}
			readLock.Unlock()

			// do gethyperlink for all children
			if currentDepth > 1 {
				threadCount = 0
				for _, node := range currentNearby {

					readLock.Lock()
					readCount += 1
					if readCount == 1 {
						writeLock.Lock()
					}
					readLock.Unlock()
					_, nearbyExist := (*nearbyNode)[node]
					readLock.Lock()
					readCount -= 1
					if readCount == 0 {
						writeLock.Unlock()
					}
					readLock.Unlock()

					if !nearbyExist {
						threadLock.Lock()
						if threadCount >= 250 {
							wg.Wait()
							time.Sleep(time.Second)
							threadCount = 0
						}
						threadLock.Unlock()
						wg.Add(1)
						threadCount += 1
						go getHyperlinks(node, nearbyNode)
					}
				}
				wg.Wait()
			}

			// fmt.Println(currentNearby)
			for _, node := range currentNearby {
				_, PathExist := (*closestDist)[node]
				if !PathExist {
					(*closestDist)[node] = (*closestDist)[source] + 1
				}
				if (*closestDist)[node] >= (*closestDist)[source]+1 {
					if (*closestDist)[node] > (*closestDist)[source]+1 {
						(*closestDist)[node] = (*closestDist)[source] + 1
						(*parent)[node] = []string{}
					}
					(*parent)[node] = append((*parent)[node], source)
					if target == node {
						(*nodeVisited)[node] = true

					} else if !(*nodeVisited)[node] {
						dls(node, target, currentDepth-1, maxDepth, nearbyNode, parent, nodeVisited, childVisited, closestDist)
					}
				}
				// fmt.Println("Node: ", node)
				// fmt.Println("pathLength: ", closestDist[node])
			}
			(*childVisited)[source] = true
		}
		(*nodeVisited)[source] = false
	}
}

func eliminateUnnecessarySolution(source string, target string, parent Solution, closestDist SolutionDistance) (Solution, SolutionDistance) {
	var solution Solution
	var solutionDistance SolutionDistance
	var queue deque.Deque[string]
	var visited map[string]bool
	var currentNode string

	solution = make(Solution)
	solutionDistance = make(SolutionDistance)
	visited = make(map[string]bool)

	queue.PushBack(target)
	for !(queue.Len() == 0) {
		currentNode = queue.PopBack()
		if !visited[currentNode] {
			solution[currentNode] = parent[currentNode]
			solutionDistance[currentNode] = closestDist[currentNode]
			for _, currentNearby := range parent[currentNode] {
				queue.PushBack(currentNearby)
			}
			visited[currentNode] = true
		}
	}
	return solution, solutionDistance
}

func IdsProccess(source string, target string, maxDepth int, nearbyNode *map[string][]string) (Solution, SolutionDistance) {
	nodeVisited := make(map[string]bool)
	childVisited := make(map[string]bool)
	closestDist := make(map[string]int)
	closestDist[source] = 0
	visitedCount = 0
	parent := make(map[string][]string) //menunjukkan nilai parent
	dls(source, target, maxDepth, maxDepth, nearbyNode, &parent, &nodeVisited, &childVisited, &closestDist)
	if !nodeVisited[target] && maxDepth < 10 {
		maxDepth += 1
		return IdsProccess(source, target, maxDepth+1, nearbyNode)
	} else {
		return eliminateUnnecessarySolution(source, target, parent, closestDist)
	}
}

func (solution Solution) PrintParent(current string, target string, firstRecursive bool, AlreadyPrinted map[string]bool) {
	if firstRecursive {
		AlreadyPrinted = make(map[string]bool)
		current = TitleToUrl(current)
		target = TitleToUrl(target)
	}
	if (AlreadyPrinted)[current] == false {
		(AlreadyPrinted)[current] = true
		fmt.Println("Node: ", current)
		if current != target {
			fmt.Println("parentnya: ", solution[current])
			for _, node := range solution[current] {
				solution.PrintParent(node, target, false, AlreadyPrinted)
			}
		}
	}
}

func (parent Solution) PrintPerPath(current string, firstNode string, currentOutput string) {
	if current == firstNode {
		currentOutput = current + ", " + currentOutput
		fmt.Println(currentOutput)
	} else {
		for _, currentParent := range parent[current] {
			if currentOutput != "" {
				parent.PrintPerPath(currentParent, firstNode, current+", "+currentOutput)
			} else {
				parent.PrintPerPath(currentParent, firstNode, current)
			}
		}
	}
}

func GetIdsResult(source string, target string) (Solution, int64, int, int) {
	sourceUrl := TitleToUrl(source)
	targetUrl := TitleToUrl(target)
	nearbyNode := make(map[string][]string)
	start := time.Now()
	wg.Add(1)
	go getHyperlinks(sourceUrl, &nearbyNode)
	wg.Wait()
	solution, solutionDistance := IdsProccess(sourceUrl, targetUrl, 0, &nearbyNode)
	execTime := time.Since(start).Milliseconds()
	return solution, execTime, int(visitedCount), solutionDistance[targetUrl]
}

// use example
// func main() {
// 	var hasil ids.Solution
// 	source := "Ostrich"
// 	target := "Camel"
// 	hasil, hasilJarak, _, _ := ids.GetIdsResult(source, target)
// 	fmt.Println(hasil)
// 	fmt.Println(hasilJarak)
// }
